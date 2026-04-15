package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestHTTPMetrics creates httpMetrics backed by a fresh registry to avoid
// cross-test pollution with the default global registry.
func newTestHTTPMetrics(reg *prometheus.Registry) *httpMetrics {
	f := promauto.With(reg)
	return &httpMetrics{
		requests: f.NewCounterVec(
			prometheus.CounterOpts{Name: "test_requests_total"},
			[]string{"method", "path", "status_class"},
		),
		duration: f.NewHistogramVec(
			prometheus.HistogramOpts{Name: "test_duration_seconds"},
			[]string{"method", "path"},
		),
		inflight: f.NewGauge(prometheus.GaugeOpts{Name: "test_inflight"}),
	}
}

func TestHTTPMetrics_2xx(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newTestHTTPMetrics(reg)
	handler := httpMetricsMiddleware(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/test", "2xx"))
	if got != 1 {
		t.Errorf("requests_total{2xx}: want 1, got %v", got)
	}
}

func TestHTTPMetrics_4xx(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newTestHTTPMetrics(reg)
	handler := httpMetricsMiddleware(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := testutil.ToFloat64(m.requests.WithLabelValues(http.MethodGet, "/missing", "4xx"))
	if got != 1 {
		t.Errorf("requests_total{4xx}: want 1, got %v", got)
	}
}

func TestHTTPMetrics_InflightZeroAfterRequest(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := newTestHTTPMetrics(reg)
	handler := httpMetricsMiddleware(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := testutil.ToFloat64(m.inflight); got != 0 {
		t.Errorf("inflight: want 0 after request completion, got %v", got)
	}
}

func TestStatusClass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code int
		want string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{299, "2xx"},
		{301, "3xx"},
		{302, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{499, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
		{0, "unknown"},
		{99, "unknown"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("code_%d", tc.code), func(t *testing.T) {
			t.Parallel()
			if got := statusClass(tc.code); got != tc.want {
				t.Errorf("statusClass(%d): want %q, got %q", tc.code, tc.want, got)
			}
		})
	}
}

func TestRoutePattern_FallbackToURLPath(t *testing.T) {
	t.Parallel()

	// No chi context in this request — should fall back to the URL path.
	ctx := context.Background()
	got := routePattern(ctx, "/api/v1/users/some-uuid")
	if got != "/api/v1/users/some-uuid" {
		t.Errorf("want fallback URL path, got %q", got)
	}
}

func TestRoutePattern_UsesChiPattern(t *testing.T) {
	t.Parallel()

	var got string
	r := chi.NewRouter()
	r.Get("/api/v1/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		got = routePattern(req.Context(), req.URL.Path)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/users/6ba7b810-9dad-11d1-80b4-00c04fd430c8", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	const want = "/api/v1/users/{id}"
	if got != want {
		t.Errorf("routePattern: want %q, got %q", want, got)
	}
	if got == "/api/v1/users/6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Error("UUID leaked into route pattern label — cardinality violation")
	}
}

func TestRecoveringPanicCounter_IncrementsPanicsAndReturns500(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	f := promauto.With(reg)
	panics := f.NewCounter(prometheus.CounterOpts{Name: "test_panics_total"})
	m := &panicMetrics{panics: panics}

	handler := recoveringPanicMiddleware(m)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rr.Code)
	}
	if got := testutil.ToFloat64(panics); got != 1 {
		t.Errorf("panics_total: want 1, got %v", got)
	}
}
