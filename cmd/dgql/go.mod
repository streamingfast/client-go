module github.com/streamingfast/client-go/cmd/dgql

go 1.15

require (
	github.com/lithammer/dedent v1.1.0
	github.com/spf13/cobra v1.1.3
	github.com/streamingfast/cli v0.0.3-0.20211104095852-a63780ebe092
	github.com/streamingfast/client-go v0.0.0-20210429191755-f6e50c5f63ba
	github.com/streamingfast/logging v0.0.0-20211130053023-cb3ab619b508
	go.uber.org/zap v1.16.0
)

replace github.com/streamingfast/client-go => ../..
