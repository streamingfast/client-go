module github.com/dfuse-io/client-go/cmd/dgql

go 1.15

require (
	github.com/dfuse-io/cli v0.0.2
	github.com/dfuse-io/client-go v0.0.0-20210429191755-f6e50c5f63ba
	github.com/dfuse-io/logging v0.0.0-20210518215502-2d920b2ad1f2
	github.com/lithammer/dedent v1.1.0
	github.com/spf13/cobra v1.1.3
	go.uber.org/zap v1.16.0
)

replace github.com/dfuse-io/client-go => ../..
