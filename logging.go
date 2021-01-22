package dfuse

import "github.com/dfuse-io/logging"

var traceEnabled = logging.IsTraceEnabled("dfuse-client", "github.com/dfuse-io/client-go")
var zlog = logging.NewNopLogger("dfuse-client", "github.com/dfuse-io/client-go")
