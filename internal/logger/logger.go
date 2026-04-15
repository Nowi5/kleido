// Package logger provides a structured slog.Logger with secret redaction
// and request-scoped context helpers.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// redactedKeys is the set of attribute key names whose values are replaced with
// "[REDACTED]" before being emitted. Matching is case-insensitive.
var redactedKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"secret":        {},
	"authorization": {},
	"api_key":       {},
	"refresh_token": {},
	"credit_card":   {},
	"ssn":           {},
}

// contextKey is the unexported type used as the context key for the logger.
// Using a dedicated type prevents collisions with other packages.
type contextKey struct{}

// New creates a configured *slog.Logger.
//
//   - level:       one of "debug", "info", "warn", "error" (case-insensitive; defaults to "info")
//   - env:         "development" → TextHandler; anything else → JSONHandler
//   - serviceName: included as a base attribute on every log record
//   - version:     included as a base attribute on every log record
func New(level, env, serviceName, version string) *slog.Logger {
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

	isDev := strings.EqualFold(env, "development")

	opts := &slog.HandlerOptions{
		Level:       lvl,
		AddSource:   !isDev,
		ReplaceAttr: replaceAttr,
	}

	var handler slog.Handler
	if isDev {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("version", version),
		slog.String("env", env),
	)
}

// replaceAttr is the slog ReplaceAttr function that redacts sensitive values.
func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindLogValuer {
		return a
	}
	if _, sensitive := redactedKeys[strings.ToLower(a.Key)]; sensitive {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

// WithContext returns a copy of ctx with the logger stored inside.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger stored in ctx by WithContext.
// If no logger is found, slog.Default() is returned.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// traceHandler is a slog.Handler that injects trace_id and span_id from the
// active OTel span into every log record emitted via *Context methods.
// Records without an active span pass through unchanged.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

// WrapWithOTel wraps base with a handler that injects trace_id and span_id
// from the active OTel span into every log record emitted via *Context methods.
// Call this in main() after telemetry.Setup() has initialized the TracerProvider.
// Only call this when OTel is enabled — the trace injection is a no-op when no
// span is active but adds a small overhead on every log call.
func WrapWithOTel(base *slog.Logger, _ string) *slog.Logger {
	return slog.New(&traceHandler{Handler: base.Handler()})
}
