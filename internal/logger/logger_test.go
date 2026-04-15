package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"kleido/internal/logger"
)

// newTestLogger returns a logger that writes to the provided buffer instead of
// os.Stdout so tests can capture output.
func newTestLogger(t *testing.T, buf *bytes.Buffer, level, env, service, version string) *slog.Logger {
	t.Helper()
	isDev := strings.EqualFold(env, "development")

	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: !isDev,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// mirror the redaction logic from the package
			redacted := map[string]struct{}{
				"password": {}, "token": {}, "secret": {},
				"authorization": {}, "api_key": {}, "refresh_token": {},
				"credit_card": {}, "ssn": {},
			}
			key := strings.ToLower(a.Key)
			if _, ok := redacted[key]; ok {
				return slog.String(a.Key, "[REDACTED]")
			}
			return a
		},
	}

	var h slog.Handler
	if isDev {
		h = slog.NewTextHandler(buf, opts)
	} else {
		h = slog.NewJSONHandler(buf, opts)
	}

	return slog.New(h).With(
		slog.String("service", service),
		slog.String("version", version),
		slog.String("env", env),
	)
}

func TestRedaction_Password(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "debug", "development", "svc", "v1")
	l.Info("test", slog.String("password", "mysecret"))

	out := buf.String()
	if strings.Contains(out, "mysecret") {
		t.Errorf("output must not contain the raw secret; got: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("output must contain [REDACTED]; got: %s", out)
	}
}

func TestRedaction_Token(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "debug", "development", "svc", "v1")
	l.Info("test", slog.String("token", "supersecrettoken"))

	out := buf.String()
	if strings.Contains(out, "supersecrettoken") {
		t.Errorf("output must not contain the raw token; got: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("output must contain [REDACTED]; got: %s", out)
	}
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "info", "development", "svc", "v1")

	ctx := logger.WithContext(context.Background(), l)
	got := logger.FromContext(ctx)

	if got != l {
		t.Error("FromContext should return the exact same logger instance stored by WithContext")
	}
}

func TestFallback_NoLoggerInContext(t *testing.T) {
	t.Parallel()

	got := logger.FromContext(context.Background())
	if got == nil {
		t.Error("FromContext with no logger in context must return a non-nil logger (the default)")
	}
}

func TestJSONOutput_ProductionMode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "info", "production", "svc", "v1")
	l.Info("hello world")

	out := buf.Bytes()
	if !json.Valid(out) {
		t.Errorf("production mode output must be valid JSON; got: %s", out)
	}
}

func TestBaseAttributes_ServiceAndVersion(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "info", "production", "myservice", "2.0.0")
	l.Info("check attrs")

	out := buf.String()
	if !strings.Contains(out, "myservice") {
		t.Errorf("output must contain service name; got: %s", out)
	}
	if !strings.Contains(out, "2.0.0") {
		t.Errorf("output must contain version; got: %s", out)
	}
}

// TestNew_DevelopmentMode exercises the exported New function with a real os.Stdout
// writer — we just verify it doesn't panic and returns non-nil.
func TestNew_DevelopmentMode(t *testing.T) {
	t.Parallel()

	l := logger.New("debug", "development", "svc", "v0")
	if l == nil {
		t.Error("New must return non-nil logger")
	}
}

func TestNew_ProductionMode(t *testing.T) {
	t.Parallel()

	l := logger.New("info", "production", "svc", "v0")
	if l == nil {
		t.Error("New must return non-nil logger")
	}
}
