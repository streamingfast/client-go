package dfuse_test

import (
	"context"
	"fmt"
	"io"
	"os"

	dfuse "github.com/streamingfast/client-go"
)

func ExampleExperimentalClient_RawGraphQL() {
	client, err := dfuse.NewClient("testnet.eos.dfuse.io", os.Getenv("DFUSE_API_KEY"))
	if err != nil {
		panic(fmt.Errorf("new dfuse client: %w", err))
	}

	// The experimental interface must be explicitely cast to, no backward compatibility layer for method in there
	experimentalClient := client.(dfuse.ExperimentalClient)

	document := graphqlDocumentFromFile("example_graphql_subscription.graphql")
	stream, err := experimentalClient.RawGraphQL(context.Background(), document, dfuse.GraphQLVariables{
		"query":  "-action:onblock",
		"cursor": "",
		"limit":  3,
	})
	if err != nil {
		panic(fmt.Errorf("graphql subscription: %w", err))
	}

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				fmt.Println("completed stream")
				return
			}

			panic(fmt.Errorf("stream error: %w", err))
		}

		fmt.Println(response.Data, response.Errors)
	}
}
