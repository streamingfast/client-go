package dfuse

import (
	"github.com/streamingfast/logging"
	"go.uber.org/zap"
)

var zlog = zap.NewNop()
var tracer = logging.LibraryLogger("client-go", "github.com/streamingfast/client-go", &zlog)
