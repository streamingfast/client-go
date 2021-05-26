package dfuse

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	pbgraphql "github.com/dfuse-io/pbgo/dfuse/graphql/v1"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type GraphQLStream interface {
	pbgraphql.GraphQL_ExecuteClient
}

func (c *client) GraphQLQuery(ctx context.Context, document string, opts ...GraphQLOption) (*pbgraphql.Response, error) {
	options := graphqlOptions{}
	for _, opt := range opts {
		opt.apply(&options)
	}

	graphql, err := c.getGraphqlClient()
	if err != nil {
		return nil, fmt.Errorf("get graphql client: %w", err)
	}

	var callOptions []grpc.CallOption
	if c.authenticated {
		tokenInfo, err := c.GetAPITokenInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("get api token: %w", err)
		}

		callOptions = append(callOptions, grpc.PerRPCCredentials(
			oauth.NewOauthAccess(&oauth2.Token{AccessToken: tokenInfo.Token, TokenType: "Bearer"})),
		)
	}

	request := &pbgraphql.Request{Query: document}
	if len(options.variables) > 0 {
		request.Variables, err = structpb.NewStruct(options.variables)
		if err != nil {
			return nil, fmt.Errorf("invalid variables: %w", err)
		}
	}

	subCtx, cancelRequest := context.WithCancel(ctx)
	defer cancelRequest()

	stream, err := graphql.Execute(subCtx, request, callOptions...)
	if err != nil {
		return nil, fmt.Errorf("graphql execute: %w", err)
	}

	response, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return response, nil
}

func (c *client) GraphQLSubscription(ctx context.Context, document string, opts ...GraphQLOption) (GraphQLStream, error) {
	stream, err := c.prepareGRPCCall(ctx, "subscription", document, opts)
	if err != nil {
		return nil, err
	}

	return &graphqlStream{stream, c.logger, nil}, nil
}

type graphqlStream struct {
	pbgraphql.GraphQL_ExecuteClient
	logger  *zap.Logger
	lastErr error
}

// LastErr returns that last error we seen in this stream. This special stream traps transient
// errors and automatically reconnects ensure a never ending flow of data so the consumer of
// client-go does not need to deal with this.
func (s *graphqlStream) LastErr() error {
	return s.lastErr
}

func (s *graphqlStream) Recv() (*pbgraphql.Response, error) {
	if tracer.Enabled() {
		zlog.Debug("about to request to receive a graphql response from gRPC stream")
	}

	response, err := s.GraphQL_ExecuteClient.Recv()
	if err == nil {
		if tracer.Enabled() {
			zlog.Debug("forwarding graphql received response from gRPC stream to consumer")
		}

		return response, nil
	}

	// It's unclear, but when the context of the stream is canceled, the `Recv` on the stream client
	// returns io.EOF, if there is a context error, we must forward it here right away
	ctxErr := s.Context().Err()
	if ctxErr != nil {
		zlog.Debug("graphql gRPC initial stream context has been canceled or timed out, returning its error right away", zap.Error(ctxErr))
		return nil, ctxErr
	}

	if err == io.EOF {
		zlog.Debug("graphql gRPC stream completed")
		return nil, io.EOF
	}

	s.lastErr = err
	isTransient := s.isTransientError(err)
	if !isTransient {
		zlog.Debug("graphql stream permanent error occurs, giving up", zap.Error(err))
		return nil, err
	}

	// FIXME: Once we have a correct enough group of unit tests, refactor this code as it's mostly
	//        the same as the part above this comment expect for a few differences that arise when
	//        running for the first time vs running as a "retry".
	zlog.Debug("a graphql stream transient error occurs, re-trying until we succeed", zap.Error(err))
	for {
		response, err := s.GraphQL_ExecuteClient.Recv()
		if err == nil {
			if tracer.Enabled() {
				zlog.Debug("retry succeeded, forwarding graphql received message from gRPC stream to consumer")
			}

			return response, nil
		}

		// It's unclear, but when the context of the stream is canceled, the `Recv` on the stream client
		// returns io.EOF, if there is a context error, we must forward it here right away
		ctxErr := s.Context().Err()
		if ctxErr != nil {
			zlog.Debug("graphql gRPC retried stream context has been canceled or timed out, returning its error right away", zap.Error(ctxErr))
			return nil, ctxErr
		}

		if err == io.EOF {
			zlog.Debug("graphql gRPC stream completed while retrying")
			return nil, io.EOF
		}

		// FIXME: This doesn't work because the receiver of the call is non-pointer ... hmmm
		s.lastErr = err
		isTransient := s.isTransientError(err)
		if !isTransient {
			zlog.Debug("a graphql stream permanent error occurs while retrying, giving up", zap.Error(err))
			return nil, err
		}

		zlog.Debug("a graphql stream transient error occurs while retrying, let's continue", zap.Error(err))
	}
}

func (s *graphqlStream) isTransientError(err error) bool {
	switch status.Code(err) {
	// Weird case where an error would have the OK code, warn & reconnect since we assume it's something wrong
	case codes.OK:
		s.logger.Warn("the error has code OK, this is really unexpected in an error case, assuming we need to re-connect", zap.Error(err))
		return true

	// Clear cases of permanent error that requires user intervention and for which we will NOT reconnect
	case codes.Canceled,
		codes.InvalidArgument,
		codes.DeadlineExceeded,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unimplemented,
		codes.Unauthenticated:
		return false

	// Potential permanent error for which I'm not 100% sure
	case codes.OutOfRange:
		return false

	// Potential transient error for which I'm not 100% sure
	case codes.DataLoss,
		codes.ResourceExhausted,
		codes.FailedPrecondition:
		return true

	// Clear cases of transient error that we should reconnect
	case codes.Unknown,
		codes.Aborted,
		codes.Internal,
		codes.Unavailable:
		return true
	default:
		return false
	}
}

func (c *client) RawGraphQL(ctx context.Context, document string, opts ...GraphQLOption) (pbgraphql.GraphQL_ExecuteClient, error) {
	return c.prepareGRPCCall(ctx, "raw", document, opts)
}

func (c *client) prepareGRPCCall(
	ctx context.Context,
	tag string,
	document string,
	opts []GraphQLOption,
) (stream pbgraphql.GraphQL_ExecuteClient, err error) {
	options := graphqlOptions{}
	for _, opt := range opts {
		opt.apply(&options)
	}

	graphql, err := c.getGraphqlClient()
	if err != nil {
		return nil, fmt.Errorf("get graphql client: %w", err)
	}

	var callOptions []grpc.CallOption
	if c.authenticated {
		tokenInfo, err := c.GetAPITokenInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("get api token: %w", err)
		}

		callOptions = append(callOptions, grpc.PerRPCCredentials(
			oauth.NewOauthAccess(&oauth2.Token{AccessToken: tokenInfo.Token, TokenType: "Bearer"})),
		)
	}

	request := &pbgraphql.Request{Query: document}
	if len(options.variables) > 0 {
		request.Variables, err = structpb.NewStruct(options.variables)
		if err != nil {
			return nil, fmt.Errorf("invalid variables: %w", err)
		}
	}

	zlog.Debug("executing graphql request over gRPC", zap.Reflect("request", request))
	stream, err = graphql.Execute(ctx, request, callOptions...)
	if err != nil {
		return nil, fmt.Errorf("graphql execute %s: %w", tag, err)
	}

	return stream, nil
}

func (c *client) getGraphqlClient() (pbgraphql.GraphQLClient, error) {
	_, err := c.getGRPCConn()
	if err != nil {
		return nil, fmt.Errorf("get grpc connection: %w", err)
	}

	return c.grpcGraphqlClient, nil
}

func (c *client) getGRPCConn() (*grpc.ClientConn, error) {
	if c.grpcConn != nil {
		return c.grpcConn, nil
	}

	c.grpcLock.Lock()
	defer c.grpcLock.Unlock()

	// It might have been set after we obtain lock, return right away if it's the case
	if c.grpcConn != nil {
		return c.grpcConn, nil
	}

	var err error
	c.grpcConn, err = newGRPCClient(c.grpcAddr, c.grpcDialOptions...)
	if err == nil {
		c.grpcGraphqlClient = pbgraphql.NewGraphQLClient(c.grpcConn)
	}

	return c.grpcConn, err
}

type GraphQLDocument interface {
	Load(ctx context.Context) (string, error)
}

type GraphQLStringDocument string

func (d GraphQLStringDocument) Load(ctx context.Context) (string, error) {
	return string(d), nil
}

type GraphQLFileDocument string

func (d GraphQLFileDocument) Load(ctx context.Context) (string, error) {
	content, err := ioutil.ReadFile(string(d))
	return string(content), err
}

type GraphQLOption interface {
	apply(o *graphqlOptions)
}

// GraphQLVariables option to pass
type GraphQLVariables map[string]interface{}

func (f GraphQLVariables) apply(o *graphqlOptions) {
	if o.variables == nil {
		o.variables = map[string]interface{}{}
	}

	for key, value := range f {
		o.variables[key] = value
	}
}

type graphqlOptions struct {
	variables map[string]interface{}
}

type graphqlOptionFunc func(o *graphqlOptions)

func (f graphqlOptionFunc) apply(o *graphqlOptions) {
	f(o)
}
