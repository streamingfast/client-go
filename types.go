package dfuse

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

type apiKey string

func (a apiKey) String() string {
	if a != "" {
		in := string(a)

		return in[0:uint64(math.Min(float64(len(a)), 16))]
	}

	return "<unset>"
}

type unixTimestamp time.Time

func (t unixTimestamp) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(time.Time(t).Unix(), 10)), nil
}

func (t *unixTimestamp) UnmarshalJSON(data []byte) error {
	var timestamp int64
	if err := json.Unmarshal(data, &timestamp); err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	*t = unixTimestamp(time.Unix(int64(timestamp), 0))
	return nil
}

// func stringsElide(in string, startCharCount, endCharCount int) string {
// 	// FIXME: Deal with characters and no byte, which is the case by default for len and slice [a:b] notations
// 	byteCount := len(in)
// 	charCount := startCharCount + endCharCount + 8 // There must be at least 8 characters betwen both boundary otherwise we assume not safe to show

// 	if byteCount < charCount {
// 		if byteCount < 8  {
// 			return "<redacted>"
// 		}

// 		return in[0:uint64(math.Min(float64(len(a)), 16))]
// 	}
// }
