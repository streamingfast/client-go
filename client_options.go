package dfuse

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ClientOption interface {
	apply(o *clientOptions)
}

type clientOptions struct {
	apiTokenStore   APITokenStore
	authURL         string
	grpcPort        int
	insecure        bool
	plainText       bool
	unauthenticated bool
	logger          *zap.Logger
}

func (c *clientOptions) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	apiTokenStore := "<unset>"
	if c.apiTokenStore != nil {
		apiTokenStore = c.apiTokenStore.String()
	}

	encoder.AddString("api_token_store", apiTokenStore)
	encoder.AddString("auth_url", c.authURL)
	encoder.AddInt("grpc_port", c.grpcPort)
	encoder.AddBool("insecure", c.insecure)
	encoder.AddBool("plain_text", c.plainText)
	encoder.AddBool("unauthenticated", c.unauthenticated)

	return nil
}

var portSuffixRegex = regexp.MustCompile(":[0-9]{2,5}$")

func (o *clientOptions) newClient(network string, apiKey string) (*client, error) {
	logger := o.logger
	if logger == nil {
		logger = zlog
	}

	if o.authURL == "" {
		o.authURL = "https://auth.dfuse.io"
	}

	logger.Debug("about to create new client with options", zap.Object("options", o))

	authURL, err := url.Parse(o.authURL)
	if err != nil {
		return nil, fmt.Errorf("invalid auth URL %q: %w", o.authURL, err)
	}

	authURL.Path = path.Join(authURL.Path, "v1", "auth", "issue")

	c := &client{
		apiKey:        apiKey,
		apiTokenStore: o.apiTokenStore,
		authenticated: !o.unauthenticated,
		authClient:    &http.Client{Timeout: 10 * time.Second},
		authIssueURL:  authURL.String(),
		logger:        logger,
	}

	if c.apiTokenStore == nil {
		c.apiTokenStore = NewOnDiskAPITokenStore(apiKey)
	}

	c.grpcAddr = network
	if !portSuffixRegex.MatchString(c.grpcAddr) {
		// Explicitely defined, use it
		if o.grpcPort != 0 {
			c.grpcAddr += ":" + strconv.FormatInt(int64(o.grpcPort), 10)
		} else if o.plainText {
			c.grpcAddr += ":9000"
		} else {
			c.grpcAddr += ":443"
		}
	}

	if o.plainText {
		c.grpcDialOptions = append(c.grpcDialOptions, plainTextDialOption)
	} else if o.insecure {
		c.grpcDialOptions = append(c.grpcDialOptions, insecureTLSDialOption)
	} else {
		c.grpcDialOptions = append(c.grpcDialOptions, secureTLSDialOption)
	}

	return c, nil
}

type clientOptionFunc func(o *clientOptions)

func (f clientOptionFunc) apply(o *clientOptions) {
	f(o)
}
