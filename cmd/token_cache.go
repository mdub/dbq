package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/databricks/databricks-sdk-go/credentials/u2m/cache"
	"golang.org/x/oauth2"
)

// fileTokenCache stores OAuth tokens in a JSON file. The SDK no longer ships
// a file-backed cache (it switched to in-memory in databricks-sdk-go v0.138.0),
// so dbq supplies its own to keep U2M tokens across invocations.
type fileTokenCache struct {
	path string
	mu   sync.Mutex
}

func newDefaultTokenCache() (*fileTokenCache, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locating config dir: %w", err)
	}
	return &fileTokenCache{path: filepath.Join(dir, "dbq", "token-cache.json")}, nil
}

func (c *fileTokenCache) load() (map[string]*oauth2.Token, error) {
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]*oauth2.Token{}, nil
	}
	if err != nil {
		return nil, err
	}
	tokens := map[string]*oauth2.Token{}
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", c.path, err)
	}
	return tokens, nil
}

func (c *fileTokenCache) save(tokens map[string]*oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}

func (c *fileTokenCache) Lookup(key string) (*oauth2.Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tokens, err := c.load()
	if err != nil {
		return nil, err
	}
	t, ok := tokens[key]
	if !ok {
		return nil, cache.ErrNotFound
	}
	return t, nil
}

func (c *fileTokenCache) Store(key string, t *oauth2.Token) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	tokens, err := c.load()
	if err != nil {
		return err
	}
	if t == nil {
		delete(tokens, key)
	} else {
		tokens[key] = t
	}
	return c.save(tokens)
}
