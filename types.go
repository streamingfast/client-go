package dfuse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

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
