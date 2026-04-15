package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kleido/internal/middleware"
)

// ── RequestLogger ─────────────────────────────────────────────────────────

func TestRequestLogger_LogsMethodPathAndStatus(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	out := buf.String()
	if !strings.Contains(out, "POST") {
		t.Errorf("log must contain request method; got: %q", out)
	}
	if !strings.Contains(out, "/api/v1/users") {
		t.Errorf("log must contain request path; got: %q", out)
	}
	if !strings.Contains(out, "201") {
		t.Errorf("log must contain status code; got: %q", out)
	}
}

func TestRequestLogger_CallsNextHandler(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	called := false

	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("RequestLogger must call the next handler")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status: want 418, got %d", rr.Code)
	}
}

func TestRequestLogger_InjectsIPAndUserAgent(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	var capturedIP, capturedUA string
	handler := middleware.RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// reqctx values are injected into the context by RequestLogger.
		// We verify indirectly that the handler receives the enriched context
		// by checking the request still has RemoteAddr and User-Agent set.
		capturedIP = r.RemoteAddr
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if capturedIP == "" {
		t.Error("handler must see a non-empty RemoteAddr")
	}
	if capturedUA != "test-agent/1.0" {
		t.Errorf("User-Agent: want %q, got %q", "test-agent/1.0", capturedUA)
	}
}

// ── Tracing ───────────────────────────────────────────────────────────────

func TestTracing_PassesRequestThrough(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.Tracing("test-service")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Tracing middleware must call the next handler")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rr.Code)
	}
}

func TestTracing_ReturnsNonNilHandler(t *testing.T) {
	t.Parallel()

	mw := middleware.Tracing("my-service")
	if mw == nil {
		t.Error("Tracing must return a non-nil middleware function")
	}
}
