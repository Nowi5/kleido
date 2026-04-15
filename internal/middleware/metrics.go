package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/nowi5/kleido/internal/logger"
	"github.com/nowi5/kleido/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// httpMetrics holds the Prometheus instruments used by the HTTP metrics middleware.
// Centralizing them in a struct allows tests to inject fresh registries.
type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight prometheus.Gauge
}

// panicMetrics holds the Prometheus instruments used by the panic-counter middleware.
type panicMetrics struct {
	panics prometheus.Counter
}

// HTTPMetrics returns a middleware that records request count, latency, and
// inflight gauge using the package-level metrics vars from internal/metrics.
// Place it AFTER RecoveringPanicCounter and SecurityHeaders but BEFORE any
// business handlers.
func HTTPMetrics() func(http.Handler) http.Handler {
	return httpMetricsMiddleware(&httpMetrics{
		requests: metrics.HTTPRequestsTotal,
		duration: metrics.HTTPRequestDurationSeconds,
		inflight: metrics.HTTPRequestsInflight,
	})
}

// httpMetricsMiddleware is the testable implementation of HTTPMetrics.
// Tests pass a fresh httpMetrics backed by a local prometheus.Registry.
func httpMetricsMiddleware(m *httpMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Save context before calling next so contextcheck is satisfied.
			ctx := r.Context()
			fallback := r.URL.Path

			m.inflight.Inc()
			defer m.inflight.Dec()

			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			path := routePattern(ctx, fallback)
			class := statusClass(ww.Status())
			duration := time.Since(start).Seconds()

			m.requests.WithLabelValues(r.Method, path, class).Inc()
			m.duration.WithLabelValues(r.Method, path).Observe(duration)
		})
	}
}

// RecoveringPanicCounter returns a middleware that catches panics, increments
// kleido_http_panics_total, logs the event, and returns HTTP 500.
// Use this INSTEAD OF chimiddleware.Recoverer — it is a full replacement.
func RecoveringPanicCounter() func(http.Handler) http.Handler {
	return recoveringPanicMiddleware(&panicMetrics{panics: metrics.HTTPPanicsTotal})
}

// recoveringPanicMiddleware is the testable implementation of RecoveringPanicCounter.
func recoveringPanicMiddleware(m *panicMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logger.FromContext(ctx)

			defer func() {
				if rec := recover(); rec != nil {
					m.panics.Inc()
					log.ErrorContext(ctx, "panic recovered", slog.Any("panic", rec))
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// routePattern returns the chi route pattern from ctx (e.g. "/api/v1/users/{id}").
// Falls back to fallbackPath if no chi context is present (e.g. in plain unit tests).
func routePattern(ctx context.Context, fallbackPath string) string {
	if rctx := chi.RouteContext(ctx); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return fallbackPath
}

// statusClass converts an HTTP status code into a bounded label string.
// Never returns a raw integer — cardinality is always 5 values.
func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
