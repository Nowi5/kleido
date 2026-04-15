package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kleido/pkg/configstore"

	"github.com/spf13/cobra"
)

// TestReadPassword_FromEnv verifies that the MYAPP_PASSWORD env var is used
// instead of reading from stdin (stdin path requires a real TTY and cannot
// be tested in a non-interactive environment).
func TestReadPassword_FromEnv(t *testing.T) {
	t.Setenv("MYAPP_PASSWORD", "hunter2")

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	got, err := readPassword(cmd)
	if err != nil {
		t.Fatalf("readPassword: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("password: want %q, got %q", "hunter2", got)
	}
	// Env var path returns early — no "Password: " prompt should be printed.
	if out.Len() != 0 {
		t.Errorf("expected no output when using env var, got %q", out.String())
	}
}

// TestLogout_WhenNotLoggedIn verifies that logout succeeds gracefully when
// there is no stored session — it should print "Not logged in." and return nil.
func TestLogout_WhenNotLoggedIn(t *testing.T) {
	isolate(t)

	cmd := newAuthLogoutCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logout should succeed when not logged in, got: %v", err)
	}
	if !strings.Contains(out.String(), "Not logged in.") {
		t.Errorf("expected 'Not logged in.' in output, got: %q", out.String())
	}
}

// TestLogout_ServerFailsButClearsConfig verifies that a server-side logout
// failure does not prevent the local config from being cleared. The client
// should still end up logged out locally.
func TestLogout_ServerFailsButClearsConfig(t *testing.T) {
	isolate(t)

	// Server that always fails logout — simulates a network or 5xx error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		//nolint:errcheck
		w.Write([]byte(`{"error":{"code":500,"message":"internal server error"}}`))
	}))
	defer srv.Close()

	// Store a valid session pointing at the failing test server.
	cfg := &configstore.Config{
		APIURL:      srv.URL,
		AccessToken: "tok-to-be-cleared",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newAuthLogoutCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logout should succeed even when server fails, got: %v", err)
	}

	// Local config must be cleared regardless of the server error.
	if _, authErr := checkAuth(); authErr == nil {
		t.Error("expected config to be cleared after logout, but checkAuth still succeeds")
	}

	// A warning about the server failure should appear on stderr.
	if !strings.Contains(errBuf.String(), "Warning:") {
		t.Errorf("expected warning on stderr, got: %q", errBuf.String())
	}

	// Confirmation message should still appear on stdout.
	if !strings.Contains(out.String(), "Logged out.") {
		t.Errorf("expected 'Logged out.' in output, got: %q", out.String())
	}
}

// TestLogout_Success verifies the happy path: server accepts logout,
// config is cleared, and "Logged out." is printed.
func TestLogout_Success(t *testing.T) {
	isolate(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := &configstore.Config{
		APIURL:      srv.URL,
		AccessToken: "tok-active",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := configstore.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newAuthLogoutCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, authErr := checkAuth(); authErr == nil {
		t.Error("expected config to be cleared after successful logout")
	}
	if !strings.Contains(out.String(), "Logged out.") {
		t.Errorf("expected 'Logged out.' in output, got: %q", out.String())
	}
}

func TestAuthCmd_HasLoginAndLogout(t *testing.T) {
	cmd := newAuthCmd()
	if len(cmd.Commands()) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(cmd.Commands()))
	}
	for _, sub := range []string{"login", "logout"} {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", sub)
		}
	}
}

func TestLogin_Success(t *testing.T) {
	isolate(t)

	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte(`{"access_token":"tok-secret","expires_at":"2030-12-31T23:59:59Z"}`))
	}))
	defer srv.Close()

	cmd := newAuthLoginCmd()
	cmd.Flags().Set("api-url", srv.URL)

	var stdin bytes.Buffer
	stdin.WriteString("alice@example.com\n")
	cmd.SetIn(&stdin)
	t.Setenv("MYAPP_PASSWORD", "hunter2")

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("login: %v", err)
	}

	if !strings.Contains(out.String(), "Logged in successfully.") {
		t.Errorf("expected success message, got: %q", out.String())
	}

	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedReq.Method)
	}

	cfg, err := checkAuth()
	if err != nil {
		t.Fatalf("checkAuth after login: %v", err)
	}
	if cfg.AccessToken != "tok-secret" {
		t.Errorf("token: want %q, got %q", "tok-secret", cfg.AccessToken)
	}
}

func TestLogin_APIError(t *testing.T) {
	isolate(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		//nolint:errcheck
		w.Write([]byte(`{"error":{"code":401,"message":"invalid credentials"}}`))
	}))
	defer srv.Close()

	cmd := newAuthLoginCmd()
	cmd.Flags().Set("api-url", srv.URL)

	var stdin bytes.Buffer
	stdin.WriteString("alice@example.com\n")
	cmd.SetIn(&stdin)
	t.Setenv("MYAPP_PASSWORD", "hunter2")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestLogin_EmailReadError(t *testing.T) {
	isolate(t)

	cmd := newAuthLoginCmd()

	cmd.SetIn(&errReader{})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for email read failure")
	}
	if !strings.Contains(err.Error(), "read email") {
		t.Errorf("expected 'read email' error, got: %v", err)
	}
}

func TestLogin_ConfigSaveError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/path/that/fails")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte(`{"access_token":"tok","expires_at":"2030-12-31T23:59:59Z"}`))
	}))
	defer srv.Close()

	cmd := newAuthLoginCmd()
	cmd.Flags().Set("api-url", srv.URL)

	var stdin bytes.Buffer
	stdin.WriteString("alice@example.com\n")
	cmd.SetIn(&stdin)
	t.Setenv("MYAPP_PASSWORD", "hunter2")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when config save fails")
	}
	if !strings.Contains(err.Error(), "save config") {
		t.Errorf("expected 'save config' error, got: %v", err)
	}
}

func TestLogin_WithEnvPassword(t *testing.T) {
	isolate(t)

	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte(`{"access_token":"tok-env","expires_at":"2030-12-31T23:59:59Z"}`))
	}))
	defer srv.Close()

	cmd := newAuthLoginCmd()
	cmd.Flags().Set("api-url", srv.URL)

	t.Setenv("MYAPP_PASSWORD", "env-password")

	var stdin bytes.Buffer
	stdin.WriteString("alice@example.com\n")
	cmd.SetIn(&stdin)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("login: %v", err)
	}

	if !strings.Contains(out.String(), "Logged in successfully.") {
		t.Errorf("expected success message, got: %q", out.String())
	}

	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	if _, err := checkAuth(); err != nil {
		t.Fatalf("checkAuth: %v", err)
	}
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated read error")
}
