package dfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	pbgraphql "github.com/dfuse-io/client-go/pb/dfuse/graphql/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func WithAPITokenStore(store APITokenStore) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.apiTokenStore = store })
}

func WithAuthURL(authURL string) ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.authURL = authURL })
}

func WithPlainText() ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.plainText = true })
}

// WithoutAuthentication disables API token retrieval and management assuming the
// endpoint connecting to does not require authentication.
func WithoutAuthentication() ClientOption {
	return clientOptionFunc(func(o *clientOptions) { o.unauthenticated = true })
}

type Client interface {
	GetAPITokenInfo(ctx context.Context) (*APITokenInfo, error)

	GraphQLQuery(ctx context.Context, document string, opts ...GraphQLOption) (*pbgraphql.Response, error)
	GraphQLSubscription(ctx context.Context, document string, opts ...GraphQLOption) (GraphQLStream, error)
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
	options.fillDefaults(apiKey)

	if apiKey == "" && !options.unauthenticated {
		return nil, errors.New(`invalid "apiKey" argument, must be set (if connecting to an unauthenticated instance, use 'WithoutAuthentication' option to allow and empty "apiKey" argument)`)
	}

	authURL, err := url.Parse(options.authURL)
	if err != nil {
		return nil, fmt.Errorf("invalid auth URL %q: %w", options.authURL, err)
	}

	authURL.Path = path.Join(authURL.Path, "issue")

	grpcAddr := network + ":443"
	var grpcDialOptions []grpc.DialOption

	if options.plainText {
		grpcAddr = network + ":9000"
		grpcDialOptions = append(grpcDialOptions, insecureDialOption)
	} else {
		grpcDialOptions = append(grpcDialOptions, tlsClientDialOption)
	}

	return &client{
		apiKey:          apiKey,
		apiTokenStore:   options.apiTokenStore,
		authenticated:   !options.unauthenticated,
		authClient:      &http.Client{Timeout: 10 * time.Second},
		authIssueURL:    authURL.String(),
		grpcAddr:        grpcAddr,
		grpcDialOptions: grpcDialOptions,
	}, nil
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

	if tokenInfo != nil && !tokenInfo.IsAboutToExpiry() {
		if traceEnabled {
			zlog.Debug("token info retrieved from store is set and not about to expiry, returning it", zap.Object("token_info", tokenInfo))
		}

		return tokenInfo, nil
	}

	zlog.Debug("token is either not set or about to expiry, fetching a new one from auth URL", zap.Object("token_info", tokenInfo), zap.String("auth_issue_url", c.authIssueURL))
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

type clientOptions struct {
	apiTokenStore   APITokenStore
	authURL         string
	plainText       bool
	unauthenticated bool
}

func (o *clientOptions) fillDefaults(apiKey string) {
	if o.apiTokenStore == nil {
		o.apiTokenStore = NewOnDiskAPITokenStore(apiKey)
	}

	if o.authURL == "" {
		o.authURL = "https://auth.dfuse.io/v1/auth"
	}
}

type ClientOption interface {
	apply(o *clientOptions)
}

type clientOptionFunc func(o *clientOptions)

func (f clientOptionFunc) apply(o *clientOptions) {
	f(o)
}
