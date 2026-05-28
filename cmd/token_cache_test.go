package cmd

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/databricks/databricks-sdk-go/credentials/u2m/cache"
	"golang.org/x/oauth2"
)

func TestNewDefaultTokenCache(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := newDefaultTokenCache()
	if err != nil {
		t.Fatalf("newDefaultTokenCache: %v", err)
	}
	if c.path == "" {
		t.Errorf("expected non-empty path")
	}
}

func TestFileTokenCache(t *testing.T) {
	c := &fileTokenCache{path: filepath.Join(t.TempDir(), "tokens.json")}

	if _, err := c.Lookup("missing"); !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("lookup on empty cache: got %v, want ErrNotFound", err)
	}

	tok := &oauth2.Token{
		AccessToken:  "abc",
		RefreshToken: "def",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second).UTC(),
	}
	if err := c.Store("key1", tok); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := c.Lookup("key1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.AccessToken != tok.AccessToken || got.RefreshToken != tok.RefreshToken {
		t.Errorf("tokens differ: got %+v, want %+v", got, tok)
	}
	if !got.Expiry.Equal(tok.Expiry) {
		t.Errorf("expiry differs: got %v, want %v", got.Expiry, tok.Expiry)
	}

	// Survives a re-open from disk.
	c2 := &fileTokenCache{path: c.path}
	if got, err := c2.Lookup("key1"); err != nil || got.AccessToken != "abc" {
		t.Errorf("reload: got (%+v, %v)", got, err)
	}

	// Delete on nil.
	if err := c.Store("key1", nil); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := c.Lookup("key1"); !errors.Is(err, cache.ErrNotFound) {
		t.Errorf("post-delete lookup: got %v, want ErrNotFound", err)
	}
}
