package dfuse

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestFileAPITokenStore_Get(t *testing.T) {
	tests := []struct {
		name        string
		inJSON      string
		expected    *APITokenInfo
		expectedErr error
	}{
		{
			"standard",
			`{"token":"a.b.c","expires_at":1596578457}`,
			&APITokenInfo{Token: "a.b.c", ExpiresAt: utcTime(t, "2020-08-04T22:00:57Z")},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, cleanup := tokenInfoFile(t, test.inJSON)
			defer cleanup()

			store := NewFileAPITokenStore(path)

			actual, err := store.Get(context.Background())
			if test.expectedErr == nil {
				require.NoError(t, err)
				assert.Equal(t, test.expected.Token, actual.Token)
				assert.Equal(t, test.expected.ExpiresAt, actual.ExpiresAt.UTC())
			} else {
				assert.Equal(t, test.expectedErr, err)
			}
		})
	}
}

func TestFileAPITokenStore_Set(t *testing.T) {
	tests := []struct {
		name         string
		in           *APITokenInfo
		expectedJSON string
		expectedErr  error
	}{
		{
			"standard",
			&APITokenInfo{Token: "a.b.c", ExpiresAt: utcTime(t, "2020-08-04T22:00:57Z")},
			`{"token":"a.b.c","expires_at":1596578457}`,
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, cleanup := tmpDir(t, "token")
			defer cleanup()

			path := filepath.Join(dir, "token.json")
			store := NewFileAPITokenStore(path)

			err := store.Set(context.Background(), test.in)
			if test.expectedErr == nil {
				require.NoError(t, err)

				actual, err := ioutil.ReadFile(path)
				require.NoError(t, err)

				assert.JSONEq(t, test.expectedJSON, string(actual))
			} else {
				assert.Equal(t, test.expectedErr, err)
			}
		})
	}
}

func tokenInfoFile(t *testing.T, content string) (path string, cleanup func()) {
	dir, cleanup := tmpDir(t, "token")
	path = filepath.Join(dir, "token.json")

	err := ioutil.WriteFile(path, []byte(content), os.ModePerm)
	require.NoError(t, err)

	return path, cleanup
}

func tmpDir(t *testing.T, name string) (dir string, cleanup func()) {
	var err error
	dir, err = ioutil.TempDir("", name)
	require.NoError(t, err)

	return dir, func() { os.RemoveAll(dir) }
}

func utcTime(t *testing.T, in string) time.Time {
	out, err := time.Parse(time.RFC3339, in)
	require.NoError(t, err)

	return out
}
