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

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Create the kleido config directory and write syntactically invalid YAML.
	cfgDir := filepath.Join(dir, "kleido")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// An unclosed flow mapping is guaranteed to be rejected by the YAML parser.
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("{invalid yaml"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := configstore.Load()
	if err == nil {
		t.Error("Load with invalid YAML should return an error, got nil")
	}
}

func TestPath_ReturnsConfigYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	p, err := configstore.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(tmp, "kleido", "config.yaml")
	if p != want {
		t.Errorf("Path: want %q, got %q", want, p)
	}
}

// TestDir_FallsBackToHomeDir verifies the UserHomeDir fallback when XDG_CONFIG_HOME is unset.
func TestDir_FallsBackToHomeDir(t *testing.T) {
	// Explicitly clear XDG_CONFIG_HOME so Dir uses os.UserHomeDir instead.
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := configstore.Dir()
	if err != nil {
		t.Fatalf("Dir without XDG_CONFIG_HOME: %v", err)
	}
	// The result must end in .config/kleido (or \kleido on Windows).
	if filepath.Base(dir) != "kleido" {
		t.Errorf("Dir: expected last segment to be 'kleido', got %q", dir)
	}
}

// TestSave_MkdirFailure verifies Save returns an error when the config
// directory cannot be created (a file blocks the path).
func TestSave_MkdirFailure(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)

	// Place a regular file where the "kleido" directory should be created.
	// os.MkdirAll will fail because it cannot turn a file into a directory.
	blocker := filepath.Join(base, "kleido")
	if err := os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatalf("setup blocker file: %v", err)
	}

	err := configstore.Save(&configstore.Config{AccessToken: "tok"})
	if err == nil {
		t.Error("Save should return an error when the config directory cannot be created")
	}
}

// TestClearToken_LoadError verifies ClearToken propagates a Load error when
// the config file contains invalid YAML.
func TestClearToken_LoadError(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)

	// Write invalid YAML so Load fails.
	cfgDir := filepath.Join(base, "kleido")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("{invalid yaml"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configstore.ClearToken(); err == nil {
		t.Error("ClearToken should return an error when Load fails (invalid YAML)")
	}
}
