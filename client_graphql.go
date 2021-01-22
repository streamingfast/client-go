package dfuse

import (
	"context"
	"fmt"
	"io/ioutil"

	pbgraphql "github.com/dfuse-io/client-go/pb/dfuse/graphql/v1"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
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

type graphqlStream struct {
	pbgraphql.GraphQL_ExecuteClient
}

func (c *client) GraphQLSubscription(ctx context.Context, document string, opts ...GraphQLOption) (GraphQLStream, error) {
	return nil, nil
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
