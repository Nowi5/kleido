package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kleido/internal/middleware"
	"kleido/internal/reqctx"

	"github.com/google/uuid"
)

func TestExtractSubdomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host     string
		expected string
	}{
		{"tenant1.example.com", "tenant1"},
		{"foo.bar.com", "foo"},
		{"example.com", ""},
		{"localhost:8080", ""},
		{"192.168.1.1", ""},
		{"a.b.c.com", "a"},
		{"tenant-with-dash.example.com", "tenant-with-dash"},
		{"127.0.0.1", ""},
		{"api", ""},
		{"myapp-api", ""},
		{"myapp-api:3000", ""},
		{"sub.domain.com", "sub"},
		{"deep.sub.domain.com", "deep"},
		{"a.b.c.d.example.com", "a"},
		{"test.dev.domain.co.uk", "test"},
		{"", ""},
		{"localhost", ""},
		{"api.example.com", "api"},
		{"www.example.com", "www"},
		{"sub.domain.com", "sub"},
		{"deep.sub.domain.com", "deep"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = tc.host

		result := middleware.ExtractSubdomain(req)
		if result != tc.expected {
			t.Errorf("ExtractSubdomain(%s) = %q; want %q", tc.host, result, tc.expected)
		}
	}
}

func TestTenantIDContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenantID := uuid.New()
	ctxWithTenant := reqctx.WithTenantID(ctx, tenantID)

	if reqctx.TenantIDFromContext(ctxWithTenant) != tenantID {
		t.Error("tenant ID not found in context")
	}

	if reqctx.TenantIDFromContext(ctx) != uuid.Nil {
		t.Error("expected uuid.Nil when no tenant in context")
	}
}

func TestTenantIDContextNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := reqctx.TenantIDFromContext(ctx)
	if result != uuid.Nil {
		t.Errorf("expected uuid.Nil, got %v", result)
	}
}

func TestTenantIDContextMultiple(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant1 := uuid.New()
	tenant2 := uuid.New()

	ctx1 := reqctx.WithTenantID(ctx, tenant1)
	ctx2 := reqctx.WithTenantID(ctx1, tenant2)

	if reqctx.TenantIDFromContext(ctx2) != tenant2 {
		t.Error("expected last tenant ID in context")
	}

	if reqctx.TenantIDFromContext(ctx) != uuid.Nil {
		t.Error("expected nil for original context")
	}
}

func TestRequireTenantID_Missing(t *testing.T) {
	t.Parallel()

	var called bool
	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
	if called {
		t.Error("handler should not be called")
	}
}

func TestRequireTenantID_Present(t *testing.T) {
	t.Parallel()

	var called bool
	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if !called {
		t.Error("handler should be called")
	}
}

func TestRequireTenantID_ErrorMessage(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.ServeHTTP(rr, req)

	if rr.Body.String() != "tenant_id is required\n" {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestTenantIsolation_NoTenant(t *testing.T) {
	t.Parallel()

	var called bool
	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	if called {
		t.Error("handler should not be called")
	}
}

func TestRequireTenantID_SetsCorrectContext(t *testing.T) {
	t.Parallel()

	var extractedID uuid.UUID
	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedID = reqctx.TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if extractedID != tenantID {
		t.Errorf("expected %s, got %s", tenantID, extractedID)
	}
}

func TestRequireTenantID_Chained(t *testing.T) {
	t.Parallel()

	var order []string
	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if len(order) != 1 || order[0] != "handler" {
		t.Errorf("handler not called in order: %v", order)
	}
}

func TestRequireTenantID_UuidNil(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx := reqctx.WithTenantID(context.Background(), uuid.Nil)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestRequireTenantID_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := reqctx.TenantIDFromContext(r.Context())
		w.Write([]byte(tenantID.String()))
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)

	var results []string
	for i := 0; i < 10; i++ {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		results = append(results, rr.Body.String())
	}

	for _, r := range results {
		if r != tenantID.String() {
			t.Errorf("expected %s, got %s", tenantID, r)
		}
	}
}

func TestRequireTenantID_ResponseHeaders(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "value")
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Header().Get("X-Test") != "value" {
		t.Error("custom header not preserved")
	}
}

func TestRequireTenantID_InvalidUUIDInQuery(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/?tenant_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTenantFromPath_NoSlug(t *testing.T) {
	t.Parallel()

	var called bool
	mw := middleware.TenantFromPath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || !called {
		t.Error("expected handler to be called when no slug")
	}
}

func TestRequireTenantID_BodyWritten(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		if rr.Body.Len() == 0 {
			t.Error("expected error body to be written")
		}
	}
}

func TestRequireTenantID_EmptyContext(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestRequireTenantID_AfterMultipleContextValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = context.WithValue(ctx, "key1", "value1")
	ctx = context.WithValue(ctx, "key2", "value2")
	ctx = reqctx.WithTenantID(ctx, uuid.New())

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}

func TestExtractSubdomain_IPAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host     string
		expected string
	}{
		{"10.0.0.1", ""},
		{"172.16.0.1", ""},
		{"192.168.1.1", ""},
		{"1.2.3.4", ""},
		{"255.255.255.255", ""},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = tc.host

		result := middleware.ExtractSubdomain(req)
		if result != tc.expected {
			t.Errorf("ExtractSubdomain(%s) = %q; want %q", tc.host, result, tc.expected)
		}
	}
}

func TestRequireTenantID_HttpMethods(t *testing.T) {
	t.Parallel()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)

	for _, method := range methods {
		mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequestWithContext(ctx, method, "/", nil)
		rr := httptest.NewRecorder()

		mw.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("method %s: want 200, got %d", method, rr.Code)
		}
	}
}

func TestRequireTenantID_EmptyQueryParams(t *testing.T) {
	t.Parallel()

	mw := middleware.RequireTenantID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ctx := reqctx.WithTenantID(context.Background(), tenantID)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/?foo=bar&baz=", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}