package dfuse

import (
	"context"
	"fmt"
)

var globalClient Client

func RegisterGlobal(network string, apiKey string, opts ...ClientOption) (err error) {
	globalClient, err = NewClient(network, apiKey, opts...)
	return
}

func GetAPITokenInfo(ctx context.Context) (*APITokenInfo, error) {
	return getGlobalClient("GetAPITokenInfo").GetAPITokenInfo(ctx)
}

func getGlobalClient(from string) Client {
	if globalClient == nil {
		panic(fmt.Errorf("execution of %s requires the global client instance to be set but it was not, ensure you call 'dfuse.' prior %s", from, from))
	}

	return globalClient
}
