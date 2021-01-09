package dfuse_test

import (
	"context"
	"fmt"
	"os"
	"time"

	dfuse "github.com/dfuse-io/client-go"
)

func ExampleGetAPITokenInfo() {
	if err := dfuse.RegisterGlobal("testnet.eos.dfuse.io", os.Getenv("DFUSE_API_KEY")); err != nil {
		panic(fmt.Errorf("register global dfuse client: %w", err))
	}

	tokenInfo, err := dfuse.GetAPITokenInfo(context.Background())
	if err != nil {
		panic(fmt.Errorf("get api token info: %w", err))
	}

	fmt.Println(tokenInfo.Token, tokenInfo.ExpiresAt.Format(time.RFC3339))
	// Output: ""
}
