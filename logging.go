package dfuse

import "github.com/dfuse-io/logging"

var traceEnabled = logging.IsTraceEnabled("dfuse-client", "github.com/dfuse-io/client-go")
var zlog = logging.NewSimpleLogger("dfuse-client", "github.com/dfuse-io/client-go")
