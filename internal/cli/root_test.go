package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowi5/kleido/pkg/configstore"
)

// isolate redirects the config dir to a temp directory for test isolation.
// Tests using isolate must NOT call t.Parallel() — t.Setenv and t.Parallel are
// mutually exclusive since Go 1.24.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestCheckAuth_NotLoggedIn(t *testing.T) {
	isolate(t)

	_, err := checkAuth()
	if err == nil {
		t.Fatal("expected error when no token stored")
	}
}

func TestCheckAuth_ValidToken(t *testing.T) {
	isolate(t)

	cfg := &configstore.Config{
		APIURL:      "http://localhost:8080",
		AccessToken: "tok-valid",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := checkAuth()
	if err != nil {
		t.Fatalf("checkAuth: %v", err)
	}
	if got.AccessToken != "tok-valid" {
		t.Errorf("token: want %q, got %q", "tok-valid", got.AccessToken)
	}
}

func TestCheckAuth_ExpiredToken(t *testing.T) {
	isolate(t)

	cfg := &configstore.Config{
		APIURL:      "http://localhost:8080",
		AccessToken: "tok-expired",
		ExpiresAt:   time.Now().Add(-1 * time.Hour), // in the past
	}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := checkAuth()
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestCheckAuth_ZeroExpiresAt_NeverExpires(t *testing.T) {
	isolate(t)

	cfg := &configstore.Config{
		APIURL:      "http://localhost:8080",
		AccessToken: "tok-noexpiry",
		ExpiresAt:   time.Time{}, // zero = never expires
	}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := checkAuth()
	if err != nil {
		t.Fatalf("checkAuth with zero ExpiresAt: %v", err)
	}
	if got.AccessToken != "tok-noexpiry" {
		t.Errorf("token: want %q, got %q", "tok-noexpiry", got.AccessToken)
	}
}

func TestCheckAuth_EmptyToken_ReturnsError(t *testing.T) {
	isolate(t)

	// Write a config file that has an api_url but no access_token.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "kleido", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(p, []byte("api_url: http://localhost:8080\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := checkAuth()
	if err == nil {
		t.Fatal("expected error for empty access token")
	}
}
