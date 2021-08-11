package dfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	pbgraphql "github.com/streamingfast/pbgo/dfuse/graphql/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

func WithAPITokenStore(store APITokenStore) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.apiTokenStore = store })
}

func WithAuthURL(authURL string) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.authURL = authURL })
}

// WithGRPCPort is an option that can be used to overidde all heuristics performed by the client
// to infer the gRPC port to use based on the network.
func WithGRPCPort(port int) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.grpcPort = port })
}

// WithInsecure is an option that can be used to notify the client that it should use
// an insecure TLS connection for gRPC calls. This option effectively skips all TLS certificate
// validation normally performed.
//
// This option is mutually exclusive with `WithPlainText` and resets it's value to the default
// value which is `false`.
func WithInsecure() ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.insecure = true; o.plainText = false })
}

// WithPlainText is an option that can be used to notify the client that it should use
// a plain text connection (so non-TLS) for gRPC calls.
//
// This option is mutually exclusive with `WithInsecure` and resets it's value to the default
// value which is `false`.
func WithPlainText() ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.insecure = false; o.plainText = true })
}

// WithoutAuthentication disables API token retrieval and management assuming the
// endpoint connecting to does not require authentication.
func WithoutAuthentication() ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.unauthenticated = true })
}

func WithLogger(logger *zap.Logger) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.logger = logger })
}

type Client interface {
	GetAPITokenInfo(ctx context.Context) (*APITokenInfo, error)

	GraphQLQuery(ctx context.Context, document string, opts ...GraphQLOption) (*pbgraphql.Response, error)
	GraphQLSubscription(ctx context.Context, document string, opts ...GraphQLOption) (GraphQLStream, error)
}

// ExperimentalClient is an interface implemented by the client you received when doing `NewClient` but the
// method in there are **experimental**, the API could change or removed at any moment.
//
// There is not backward compatibility policy for those methods.
type ExperimentalClient interface {
	RawGraphQL(ctx context.Context, document string, opts ...GraphQLOption) (pbgraphql.GraphQL_ExecuteClient, error)
}

func NewClient(network string, apiKey string, opts ...ClientOption) (Client, error) {
	zlog.Info("creating new client", zap.String("network", network))
	if network == "" {
		return nil, errors.New(`invalid "network" argument, must be set`)
	}

	options := &clientOptions{}
	for _, opt := range opts {
		opt.apply(options)
	}

	if apiKey == "" && !options.unauthenticated {
		return nil, errors.New(`invalid "apiKey" argument, must be set (if connecting to an unauthenticated instance, use 'WithoutAuthentication' option to allow and empty "apiKey" argument)`)
	}

	client, err := options.newClient(network, apiKey)
	if err != nil {
		return nil, err
	}

	client.logger.Debug("created dfuse client instance", zap.Object("client", client))
	return client, nil
}

// compile time check to ensure that *client struct implements Client interface
var _ Client = (*client)(nil)

type client struct {
	apiKey        string
	apiTokenStore APITokenStore

	authClient    *http.Client
	authIssueURL  string
	authenticated bool

	grpcAddr          string
	grpcDialOptions   []grpc.DialOption
	grpcConn          *grpc.ClientConn
	grpcGraphqlClient pbgraphql.GraphQLClient
	grpcLock          sync.Mutex

	logger *zap.Logger
}

func (c *client) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	apiTokenStore := "<unset>"
	if c.apiTokenStore != nil {
		apiTokenStore = c.apiTokenStore.String()
	}

	encoder.AddString("api_key", apiKey(c.apiKey).String())
	encoder.AddString("api_token_store", apiTokenStore)
	encoder.AddString("auth_issue_url", c.authIssueURL)
	encoder.AddBool("authenticated", c.authenticated)
	encoder.AddString("grpc_addr", c.grpcAddr)
	if c.grpcConn != nil {
		encoder.AddString("grpc_conn_target", c.grpcConn.Target())
	}
	encoder.AddInt("grpc_dial_option_count", len(c.grpcDialOptions))

	return nil
}

type issueTokenResponse struct {
	Token     string        `json:"token"`
	ExpiresAt unixTimestamp `json:"expires_at"`
}

func (c *client) GetAPITokenInfo(ctx context.Context) (*APITokenInfo, error) {
	tokenInfo, err := c.apiTokenStore.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("api token store get: %w", err)
	}

	if tokenInfo != nil && !tokenInfo.IsAboutToExpire() {
		if tracer.Enabled() {
			zlog.Debug("token info retrieved from store is set and not about to expire, returning it", zap.Object("token_info", tokenInfo))
		}

		return tokenInfo, nil
	}

	zlog.Debug("token is either not set or about to expire, fetching a new one from auth URL", zap.Object("token_info", tokenInfo), zap.String("auth_issue_url", c.authIssueURL))
	tokenInfo, err = c.fetchToken(ctx)
	if err != nil {
		return nil, err
	}

	zlog.Debug("token retrieved from remote storage, setting it in api token store", zap.Object("token_info", tokenInfo))
	if err := c.apiTokenStore.Set(ctx, tokenInfo); err != nil {
		return nil, fmt.Errorf("api token store set: %w", err)
	}

	return tokenInfo, nil
}

func (c *client) fetchToken(ctx context.Context) (*APITokenInfo, error) {
	entity := map[string]interface{}{"api_key": c.apiKey}
	body, _ := json.Marshal(entity)

	request, err := http.NewRequestWithContext(ctx, "POST", c.authIssueURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	response, err := c.authClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if response.StatusCode >= 400 {
		// FIXME: Deal with response body and return it somehow to consumer, for now, generic error
		answer, err := consumeBodyToString(response)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("http request failure (code %d): %s", response.StatusCode, answer)
	}

	answer := issueTokenResponse{}
	if err := consumeBodyAsJSON(response, &answer); err != nil {
		return nil, err
	}

	return &APITokenInfo{Token: answer.Token, ExpiresAt: time.Time(answer.ExpiresAt)}, nil
}

func consumeBodyAsJSON(response *http.Response, v interface{}) error {
	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("http read body as JSON: %w", err)
	}

	return nil
}

func consumeBodyToString(response *http.Response) (string, error) {
	defer response.Body.Close()
	out, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("http read body: %w", err)
	}

	return string(out), nil
}
