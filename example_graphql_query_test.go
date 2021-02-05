package dfuse_test

import (
	"context"
	"fmt"
	"os"

	dfuse "github.com/dfuse-io/client-go"
)

func ExampleClient_GraphQLQuery() {
	client, err := dfuse.NewClient("testnet.eos.dfuse.io", os.Getenv("DFUSE_API_KEY"))
	if err != nil {
		panic(fmt.Errorf("new dfuse client: %w", err))
	}

	document := graphqlDocumentFromFile("example_graphql_query.graphql")
	response, err := client.GraphQLQuery(context.Background(), document, dfuse.GraphQLVariables{
		"blockNum": 10,
	})
	if err != nil {
		panic(fmt.Errorf("graphql query: %w", err))
	}

	fmt.Println(response.Data, response.Errors)
	// Out: test
}
