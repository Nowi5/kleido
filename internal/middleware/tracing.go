package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Tracing returns middleware that starts an OTel span for each HTTP request.
// It reads W3C traceparent/tracestate headers from the incoming request (for
// distributed tracing across services) and injects the active span into the
// request context so all downstream otel.Tracer() calls create child spans.
//
// The span name is set to the chi route pattern (e.g. "GET /api/v1/users/{id}"),
// not the raw URL, to avoid high-cardinality span names.
//
// This must be the FIRST middleware in the global chain so the span context is
// available to all downstream middleware (logger, metrics, handlers).
func Tracing(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				// Use chi route pattern as the span name to avoid high cardinality.
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					if pattern := rctx.RoutePattern(); pattern != "" {
						return r.Method + " " + pattern
					}
				}
				return r.Method + " " + r.URL.Path
			}),
		)
	}
}
