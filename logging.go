package dfuse

import (
	"github.com/dfuse-io/logging"
	"go.uber.org/zap"
)

var traceEnabled = logging.IsTraceEnabled("dfuse-client", "github.com/dfuse-io/client-go")
var zlog = zap.NewNop()

func init() {
	logging.Register("github.com/dfuse-io/client-go", &zlog)
}
