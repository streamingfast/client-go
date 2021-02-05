package dfuse_test

import (
	"context"
	"fmt"
	"os"
	"time"

	dfuse "github.com/dfuse-io/client-go"
)

func ExampleClient_GetAPITokenInfo() {
	client, err := dfuse.NewClient("testnet.eos.dfuse.io", os.Getenv("DFUSE_API_KEY"))
	if err != nil {
		panic(fmt.Errorf("new dfuse client: %w", err))
	}

	tokenInfo, err := client.GetAPITokenInfo(context.Background())
	if err != nil {
		panic(fmt.Errorf("get api token info: %w", err))
	}

	fmt.Println(tokenInfo.Token, tokenInfo.ExpiresAt.Format(time.RFC3339))
}
