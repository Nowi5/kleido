// Package telemetry initializes the global OpenTelemetry TracerProvider and
// text-map propagator. It is the only place the SDK is wired up — all other
// packages obtain a tracer via otel.Tracer("kleido/<package>").
package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// traceInjectingHandler is a slog.Handler that extracts the active OTel span
// from the context and appends trace_id and span_id to every log record.
// This bridges structured logs with Jaeger traces without sending logs to OTel.
type traceInjectingHandler struct {
	slog.Handler
}

func (h *traceInjectingHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceInjectingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceInjectingHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceInjectingHandler) WithGroup(name string) slog.Handler {
	return &traceInjectingHandler{Handler: h.Handler.WithGroup(name)}
}

// Setup initializes the global OTel TracerProvider and propagator.
// It returns a shutdown function that must be deferred in main().
//
// When enabled is false, the no-op default provider is kept and a no-op
// shutdown function is returned — instrumented code runs without side effects.
//
// Sampler policy:
//   - "development": AlwaysSample — every span is recorded for local debugging.
//   - anything else: ParentBased(TraceIDRatioBased(0.1)) — 10 % head sampling in
//     staging/production, honoring upstream sampling decisions via W3C traceparent.
func Setup(ctx context.Context, serviceName, version, endpoint, env string, enabled bool) (shutdown func(context.Context) error, err error) {
	if !enabled {
		otel.SetTracerProvider(otel.GetTracerProvider()) // keep no-op default
		return func(context.Context) error { return nil }, nil
	}

	// Resource describes this service instance to the collector.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}

	// OTLP gRPC exporter — sends spans to Jaeger (or any OTLP-compatible collector).
	// NOTE: enable TLS in production via WithTLSClientConfig.
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), //nolint:staticcheck // TLS is handled by the network in dev; enable in prod
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	// Choose sampler based on environment: always-on for local dev, 10 % ratio
	// (parent-based) for staging and production.
	var sampler sdktrace.Sampler
	if strings.EqualFold(env, "development") {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global — all otel.Tracer() calls will use this provider.
	otel.SetTracerProvider(tp)

	// W3C Trace Context + Baggage propagation.
	// Reads/writes traceparent and tracestate headers on every HTTP request,
	// enabling distributed tracing across W3C-compliant services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// NewSlogHandler wraps base with a handler that injects trace_id and span_id
// from the active OTel span into every log record emitted via *Context methods.
// Call this in main() after Setup() has initialized the TracerProvider.
func NewSlogHandler(base slog.Handler, _ string) slog.Handler {
	return &traceInjectingHandler{Handler: base}
}
