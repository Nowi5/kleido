package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kleido/pkg/configstore"
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

func TestNewRootCmd_Structure(t *testing.T) {
	cmd := NewRootCmd()

	if cmd.Use != "kleido" {
		t.Errorf("Use: want %q, got %q", "kleido", cmd.Use)
	}
	if cmd.SilenceUsage != true {
		t.Error("SilenceUsage should be true")
	}
	if cmd.SilenceErrors != true {
		t.Error("SilenceErrors should be true")
	}

	subCmds := cmd.Commands()
	subCmdNames := make(map[string]bool)
	for _, c := range subCmds {
		subCmdNames[c.Name()] = true
	}

	for _, name := range []string{"auth", "users", "version", "completion"} {
		if !subCmdNames[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestNewRootCmd_OutputFlag(t *testing.T) {
	cmd := NewRootCmd()

	flag := cmd.PersistentFlags().Lookup("output")
	if flag == nil {
		t.Fatal("expected --output flag")
	}
	if flag.DefValue != "table" {
		t.Errorf("default output: want %q, got %q", "table", flag.DefValue)
	}
	if flag.Value.String() != "table" {
		t.Errorf("output value: want %q, got %q", "table", flag.Value.String())
	}
}

func TestSetVersion(t *testing.T) {
	orig := appVersion
	SetVersion("v1.2.3")
	if appVersion != "v1.2.3" {
		t.Errorf("appVersion: want %q, got %q", "v1.2.3", appVersion)
	}
	appVersion = orig
}

func TestNewClient(t *testing.T) {
	cfg := &configstore.Config{
		APIURL:      "http://localhost:8080",
		AccessToken: "tok-test",
	}

	c := newClient(cfg)
	if c == nil {
		t.Fatal("newClient returned nil")
	}
}

func TestCheckAuth_ErrorMessages(t *testing.T) {
	isolate(t)

	_, err := checkAuth()
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
	if !contains(err.Error(), "not logged in") {
		t.Errorf("expected 'not logged in' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
