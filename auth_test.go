package dfuse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPITokenInfo_IsAboutToExpire(t *testing.T) {
	tests := []struct {
		name      string
		now       string
		expiresAt string
		expected  bool
	}{
		{"expires in a day", "2020-01-01T00:00:00Z", "2020-01-02T00:00:00Z", false},
		{"expires just after threshold of 30s", "2020-01-01T00:00:00Z", "2020-01-01T00:00:31Z", false},
		{"expires right on threshold of 30s", "2020-01-01T00:00:00Z", "2020-01-01T00:00:30Z", false},
		{"expired just before threshold of 30s", "2020-01-01T00:00:00Z", "2020-01-01T00:00:29Z", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now = func() time.Time { return utcTime(t, test.now) }
			token := &APITokenInfo{Token: "a.b.c", ExpiresAt: utcTime(t, test.expiresAt)}

			assert.Equal(t, test.expected, token.IsAboutToExpiry())
		})
	}
}

func utcTime(t *testing.T, in string) time.Time {
	out, err := time.Parse(time.RFC3339, in)
	require.NoError(t, err)

	return out
}
