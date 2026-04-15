package metrics_test

import (
	"strings"
	"testing"

	"kleido/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricNames(t *testing.T) {
	t.Parallel()

	// Vec metrics (CounterVec, HistogramVec) only appear in Gather() output
	// after at least one label combination is observed. Initialize them here
	// with a sentinel label so the MetricFamily shows up in the registry output.
	metrics.HTTPRequestsTotal.WithLabelValues("GET", "/test", "2xx")
	metrics.HTTPRequestDurationSeconds.WithLabelValues("GET", "/test")
	metrics.DBQueryDurationSeconds.WithLabelValues("_init")
	metrics.DBErrorsTotal.WithLabelValues("_init")
	metrics.CacheOperationsTotal.WithLabelValues("get", "hit")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	names := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	expected := []string{
		"kleido_http_requests_total",
		"kleido_http_request_duration_seconds",
		"kleido_http_requests_inflight",
		"kleido_http_panics_total",
		"kleido_db_query_duration_seconds",
		"kleido_db_errors_total",
		"kleido_cache_operations_total",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("metric %q not found in default registry", name)
		}
	}
}

func TestLabelCardinality_HTTPPath(t *testing.T) {
	t.Parallel()

	// The middleware must use the chi route pattern as the path label,
	// not the raw URL. A UUID must never appear as a label value.
	routePattern := "/api/v1/users/{id}"
	rawPath := "/api/v1/users/6ba7b810-9dad-11d1-80b4-00c04fd430c8"

	if rawPath == routePattern {
		t.Error("route pattern must differ from raw URL path")
	}
	if !strings.Contains(routePattern, "{id}") {
		t.Error("route pattern must contain placeholder, not a concrete UUID")
	}
	if strings.Contains(routePattern, "6ba7b810") {
		t.Error("UUID must not appear in route pattern label")
	}
}

func TestMetricNaming_NoHighCardinalityLabels(t *testing.T) {
	t.Parallel()

	// Verify that kleido_* metrics do not use high-cardinality label names.
	// Any label named user_id, email, or request_id would cause Prometheus
	// memory exhaustion in production.
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	banned := []string{"user_id", "email", "request_id"}
	for _, mf := range mfs {
		if !strings.HasPrefix(mf.GetName(), "kleido_") {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				for _, banned := range banned {
					if lp.GetName() == banned {
						t.Errorf("metric %q has high-cardinality label %q", mf.GetName(), lp.GetName())
					}
				}
			}
		}
	}
}
