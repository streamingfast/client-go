package dfuse

import (
	"github.com/dfuse-io/logging"
	"go.uber.org/zap"
)

var zlog = zap.NewNop()
var tracer = logging.LibraryLogger("client-go", "github.com/dfuse-io/client-go", &zlog)
