package configstore_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"kleido/pkg/configstore"
)

// isolate redirects the config dir to a temp directory for test isolation.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	isolate(t)

	want := &configstore.Config{
		APIURL:      "http://localhost:8080",
		AccessToken: "tok-abc123",
		ExpiresAt:   time.Date(2030, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	if err := configstore.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := configstore.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIURL != want.APIURL {
		t.Errorf("APIURL: want %q, got %q", want.APIURL, got.APIURL)
	}
	if got.AccessToken != want.AccessToken {
		t.Errorf("AccessToken: want %q, got %q", want.AccessToken, got.AccessToken)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt: want %v, got %v", want.ExpiresAt, got.ExpiresAt)
	}
}

func TestLoad_MissingFile_ReturnsEmptyConfig(t *testing.T) {
	isolate(t)

	cfg, err := configstore.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config, want empty Config")
	}
	if cfg.AccessToken != "" {
		t.Errorf("AccessToken: want empty, got %q", cfg.AccessToken)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not enforced on Windows")
	}
	isolate(t)

	cfg := &configstore.Config{AccessToken: "secret"}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	p, err := configstore.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// Only owner should be able to read and write.
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions: want 0600, got %04o", info.Mode().Perm())
	}
}

func TestClearToken_ClearsAccessToken(t *testing.T) {
	isolate(t)

	// First save a config with a token.
	if err := configstore.Save(&configstore.Config{
		APIURL:      "http://localhost:9090",
		AccessToken: "token-to-clear",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := configstore.ClearToken(); err != nil {
		t.Fatalf("ClearToken: %v", err)
	}

	cfg, err := configstore.Load()
	if err != nil {
		t.Fatalf("Load after ClearToken: %v", err)
	}
	if cfg.AccessToken != "" {
		t.Errorf("AccessToken after ClearToken: want empty, got %q", cfg.AccessToken)
	}
	// Non-token fields should be preserved.
	if cfg.APIURL != "http://localhost:9090" {
		t.Errorf("APIURL should be preserved after ClearToken, got %q", cfg.APIURL)
	}
}

func TestDir_UsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := configstore.Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	want := filepath.Join(tmp, "kleido")
	if dir != want {
		t.Errorf("Dir: want %q, got %q", want, dir)
	}
}
