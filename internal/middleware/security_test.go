package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nowi5/kleido/internal/middleware"
)

func TestSecurityHeaders_ContentTypeOptions(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want %q, got %q", "nosniff", got)
	}
}

func TestSecurityHeaders_FrameOptions(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options: want %q, got %q", "DENY", got)
	}
}

func TestSecurityHeaders_HSTS_Production(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Strict-Transport-Security")
	if got == "" {
		t.Error("HSTS header must be set when isProd=true")
	}
	if got != "max-age=31536000; includeSubDomains" {
		t.Errorf("HSTS: want %q, got %q", "max-age=31536000; includeSubDomains", got)
	}
}

func TestSecurityHeaders_HSTS_Development(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS header must NOT be set when isProd=false, got %q", got)
	}
}

func TestSecurityHeaders_HandlerCalled(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("underlying handler was not called")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status: want %d, got %d", http.StatusTeapot, rr.Code)
	}
}
