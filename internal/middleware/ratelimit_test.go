package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kleido/internal/middleware"
)

// ── Mock implementations ──────────────────────────────────────────────────

type mockRateLimiter struct {
	allowed   bool
	remaining int64
	resetAt   time.Time
	err       error
}

func (m *mockRateLimiter) RateLimitAllow(_ context.Context, _ string, _ int64, _ time.Duration) (bool, int64, time.Time, error) {
	return m.allowed, m.remaining, m.resetAt, m.err
}

type mockUserRateLimiter struct {
	allowed   bool
	remaining int64
	resetAt   time.Time
	err       error
}

func (m *mockUserRateLimiter) RateLimitAllowUser(_ context.Context, _, _ string, _ int64, _ time.Duration) (bool, int64, time.Time, error) {
	return m.allowed, m.remaining, m.resetAt, m.err
}

// ── RateLimit ─────────────────────────────────────────────────────────────

func TestRateLimit_Allowed_SetsHeaders(t *testing.T) {
	t.Parallel()

	resetAt := time.Unix(9999999, 0)
	limiter := &mockRateLimiter{allowed: true, remaining: 7, resetAt: resetAt}
	handler := middleware.RateLimit(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-RateLimit-Limit"); got != "10" {
		t.Errorf("X-RateLimit-Limit: want %q, got %q", "10", got)
	}
	if got := rr.Header().Get("X-RateLimit-Remaining"); got != "7" {
		t.Errorf("X-RateLimit-Remaining: want %q, got %q", "7", got)
	}
	want := fmt.Sprintf("%d", resetAt.Unix())
	if got := rr.Header().Get("X-RateLimit-Reset"); got != want {
		t.Errorf("X-RateLimit-Reset: want %q, got %q", want, got)
	}
}

func TestRateLimit_Blocked_Returns429(t *testing.T) {
	t.Parallel()

	limiter := &mockRateLimiter{allowed: false, remaining: 0}
	called := false
	handler := middleware.RateLimit(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status: want 429, got %d", rr.Code)
	}
	if called {
		t.Error("downstream handler must not be called when rate limited")
	}
}

func TestRateLimit_FailOpen_AllowsRequest(t *testing.T) {
	t.Parallel()

	// Simulates fail-open: limiter returns error but also allowed=true,
	// matching the real Redis implementation that returns true on error.
	limiter := &mockRateLimiter{allowed: true, err: fmt.Errorf("redis unavailable")}
	handler := middleware.RateLimit(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("fail-open: want 200, got %d", rr.Code)
	}
}

// ── RateLimitUser ─────────────────────────────────────────────────────────

func TestRateLimitUser_NoUserInContext_PassesThrough(t *testing.T) {
	t.Parallel()

	limiter := &mockUserRateLimiter{} // should never be called
	called := false
	handler := middleware.RateLimitUser(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// No user ID in context — unauthenticated route.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler must be called when no user is in context (unauthenticated route)")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rr.Code)
	}
}

func TestRateLimitUser_Allowed_SetsHeaders(t *testing.T) {
	t.Parallel()

	limiter := &mockUserRateLimiter{allowed: true, remaining: 5}
	handler := middleware.RateLimitUser(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, "user-abc")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/resource", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-RateLimit-Remaining"); got != "5" {
		t.Errorf("X-RateLimit-Remaining: want %q, got %q", "5", got)
	}
}

func TestRateLimitUser_Blocked_Returns429(t *testing.T) {
	t.Parallel()

	limiter := &mockUserRateLimiter{allowed: false, remaining: 0}
	called := false
	handler := middleware.RateLimitUser(limiter, 10, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, "user-xyz")
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/resource", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status: want 429, got %d", rr.Code)
	}
	if called {
		t.Error("downstream handler must not be called when user is rate limited")
	}
}
