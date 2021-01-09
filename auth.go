package dfuse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// Ensures that interface is respected by our implementation
var _ APITokenStore = (*InMemoryAPITokenStore)(nil)
var _ APITokenStore = (*FileAPITokenStore)(nil)
var _ APITokenStore = (*OnDiskApiTokenStore)(nil)

type APITokenInfo struct {
	Token     string
	ExpiresAt time.Time
}

type APITokenStore interface {
	Get(ctx context.Context) (*APITokenInfo, error)
	Set(ctx context.Context, token *APITokenInfo) error
}

func NewInMemoryAPITokenStore() *InMemoryAPITokenStore {
	return &InMemoryAPITokenStore{}
}

// InMemoryAPITokenStore simply keeps the token in memory and serves
// it from there.
//
// It is **never** persisted and will be reset upon restart of the process
// process, leading to a new token being issued.
//
// You should try hard to use a persistent solution so that you re-use the
// same token as long as it's valid.
type InMemoryAPITokenStore struct {
	active atomic.Value
}

func (s *InMemoryAPITokenStore) Get(ctx context.Context) (*APITokenInfo, error) {
	return s.active.Load().(*APITokenInfo), nil
}

func (s *InMemoryAPITokenStore) Set(ctx context.Context, token *APITokenInfo) error {
	s.active.Store(token)
	return nil
}

func NewFileAPITokenStore(filePath string) *FileAPITokenStore {
	zlog.Info("creating file api token store", zap.String("file_path", filePath))
	return &FileAPITokenStore{filePath: filePath}
}

// FileAPITokenStore saves the active token as a JSON string in plain text in
// the given file.
type FileAPITokenStore struct {
	active   *APITokenInfo
	filePath string
	lock     sync.RWMutex
}

func (s *FileAPITokenStore) Get(ctx context.Context) (*APITokenInfo, error) {
	s.lock.RLock()
	active := s.active
	s.lock.RUnlock()

	if active != nil {
		return active, nil
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("open token file %q: %w", s.filePath, err)
	}
	defer file.Close()

	tokenInfo := &tokenInfo{}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(tokenInfo); err != nil {
		return nil, fmt.Errorf("read token file %q: %w", s.filePath, err)
	}

	s.active = &APITokenInfo{Token: tokenInfo.Token, ExpiresAt: time.Time(tokenInfo.ExpiresAt)}
	return s.active, nil
}

func (s *FileAPITokenStore) Set(ctx context.Context, token *APITokenInfo) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.active = token

	fileDir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
		return fmt.Errorf("create all directories %q: %w", fileDir, err)
	}

	file, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("create token file %q: %w", s.filePath, err)
	}
	defer file.Close()

	tokenInfo := tokenInfo{Token: token.Token, ExpiresAt: unixTimestamp(token.ExpiresAt)}
	encoder := json.NewEncoder(file)
	if err = encoder.Encode(tokenInfo); err != nil {
		return fmt.Errorf("write token file %q: %w", s.filePath, err)
	}

	return nil
}

type tokenInfo struct {
	Token     string        `json:"token"`
	ExpiresAt unixTimestamp `json:"expires_at"`
}

func NewOnDiskApiTokenStore(apiKey string) *OnDiskApiTokenStore {
	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("unable to determine home directory, use 'NewFileAPITokenStore' and specify the path manually"))
	}
	sum := shasum256StringToHex(apiKey)

	zlog.Info("creating on disk api token store", zap.String("home", homedir), zap.String("sum", sum))
	return &OnDiskApiTokenStore{FileAPITokenStore: FileAPITokenStore{
		filePath: filepath.Join(homedir, ".dfuse", sum, "token.json"),
	}}
}

// OnDiskApiTokenStore saves the active token as a JSON string in a file located
// at `~/.dfuse/<sha256-api-key>/token.json`.
//
// The directory structure is created when it does not exists.
type OnDiskApiTokenStore struct {
	FileAPITokenStore
}

func shasum256StringToHex(in string) string {
	sum := sha256.New()
	sum.Write([]byte(in))

	return hex.EncodeToString(sum.Sum(nil))
}