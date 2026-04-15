package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"kleido/pkg/apperror"
)

// RateLimiter is the subset of SessionRepository used by the per-IP rate limit middleware.
type RateLimiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, time.Time, error)
}

// UserRateLimiter is the subset of SessionRepository used by the per-user rate limit middleware.
type UserRateLimiter interface {
	RateLimitAllowUser(ctx context.Context, userID, endpoint string, limit int64, window time.Duration) (bool, int64, time.Time, error)
}

// RateLimit is a per-IP sliding-window rate limiter middleware.
// It sets X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset headers
// on every response, and returns 429 when the limit is exceeded.
// If Redis is unavailable the request is allowed through (fail open).
func RateLimit(limiter RateLimiter, limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			key := fmt.Sprintf("ip:%s", ip)

			// RateLimitAllow fails open on error — the error is intentionally discarded here.
			allowed, remaining, resetAt, _ := limiter.RateLimitAllow(r.Context(), key, limit, window) //nolint:errcheck

			h := w.Header()
			h.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			h.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

			if !allowed {
				apperror.WriteError(w, apperror.TooManyRequests())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitUser is a per-authenticated-user sliding-window rate limiter middleware.
// It must be placed AFTER the JWT middleware (requires CtxKeyUserID to be set).
// Complements the per-IP RateLimit middleware — both run independently.
// If the user ID is not in the context (unauthenticated route) or Redis is
// unavailable, the request is allowed through (fail open).
func RateLimitUser(limiter UserRateLimiter, limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(CtxKeyUserID).(string)
			if !ok || userID == "" {
				// No user in context — skip (unauthenticated route or JWT not yet verified).
				next.ServeHTTP(w, r)
				return
			}

			endpoint := routePattern(r.Context(), r.URL.Path)

			allowed, remaining, resetAt, _ := limiter.RateLimitAllowUser( //nolint:errcheck
				r.Context(), userID, endpoint, limit, window,
			)

			h := w.Header()
			h.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			h.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

			if !allowed {
				apperror.WriteError(w, apperror.TooManyRequests())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
