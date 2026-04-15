// Package metrics defines Prometheus metric instruments for the kleido API.
// All vars are package-level and registered once via promauto on package init.
// This package must not import internal/service, internal/handler, or internal/repository.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP metrics

// HTTPRequestsTotal counts completed HTTP requests.
// Labels: method, path (normalised route pattern), status_class (2xx/4xx/5xx).
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "kleido",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests completed.",
	},
	[]string{"method", "path", "status_class"},
)

// HTTPRequestDurationSeconds records the full latency of each HTTP request.
// Labels: method, path.
var HTTPRequestDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "kleido",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	},
	[]string{"method", "path"},
)

// HTTPRequestsInflight tracks how many requests are currently being processed.
var HTTPRequestsInflight = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "kleido",
	Subsystem: "http",
	Name:      "requests_inflight",
	Help:      "Number of HTTP requests currently being processed.",
})

// HTTPPanicsTotal counts requests that triggered a panic recovered by middleware.
var HTTPPanicsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "kleido",
	Subsystem: "http",
	Name:      "panics_total",
	Help:      "Total number of panics recovered by the HTTP middleware.",
})

// DB metrics

// DBQueryDurationSeconds records the latency of database operations.
// Labels: operation (e.g. "find_by_id", "list", "create", "update", "delete").
var DBQueryDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "kleido",
		Subsystem: "db",
		Name:      "query_duration_seconds",
		Help:      "Database query latency in seconds.",
		Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
	},
	[]string{"operation"},
)

// DBErrorsTotal counts database errors by operation.
// NotFound results are not counted — they are expected business-logic outcomes.
var DBErrorsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "kleido",
		Subsystem: "db",
		Name:      "errors_total",
		Help:      "Total number of database errors.",
	},
	[]string{"operation"},
)

// Cache metrics

// CacheOperationsTotal counts cache operations by type and result.
// Labels: operation ("get"/"set"/"delete"), result ("hit"/"miss"/"error"/"ok").
var CacheOperationsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "kleido",
		Subsystem: "cache",
		Name:      "operations_total",
		Help:      "Total number of cache operations.",
	},
	[]string{"operation", "result"},
)
