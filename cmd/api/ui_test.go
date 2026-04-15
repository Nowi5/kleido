package main

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDistFS returns an in-memory FS that mimics a Vite build output.
// Tests use this instead of the real embedded web/dist so they do not
// require a prior "make ui-build" to pass.
func testDistFS(_ *testing.T) fs.FS {
	return fstest.MapFS{
		"index.html":             {Data: []byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`)},
		"assets/main-abc123.js":  {Data: []byte(`console.log("app")`)},
		"assets/main-abc123.css": {Data: []byte(`body{}`)},
		"favicon.ico":            {Data: []byte(`icon`)},
	}
}

func TestRegisterUIRoutes_AssetsAreImmutable(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	registerUIRoutesWithFS(r, testDistFS(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/assets/main-abc123.js", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Cache-Control"), "immutable")
	assert.Contains(t, rr.Header().Get("Cache-Control"), "max-age=31536000")
}

func TestRegisterUIRoutes_IndexIsNoStore(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	registerUIRoutesWithFS(r, testDistFS(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Body.String(), "<!doctype html>")
}

func TestRegisterUIRoutes_UnknownPathServesIndex(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	registerUIRoutesWithFS(r, testDistFS(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/some/deep/spa-route", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	assert.Contains(t, rr.Body.String(), "<!doctype html>")
}

func TestRegisterUIRoutes_FaviconHasModerateCache(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	registerUIRoutesWithFS(r, testDistFS(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/favicon.ico", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Cache-Control"), "max-age=86400")
}

func TestRegisterUIRoutes_APINotShadowed(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()

	// Register an API route before the SPA catch-all — simulates real router setup.
	apiCalled := false
	r.Get("/api/v1/healthz", func(w http.ResponseWriter, _ *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
	})
	registerUIRoutesWithFS(r, testDistFS(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/healthz", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.True(t, apiCalled, "API route must not be shadowed by /* SPA catch-all")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRegisterUIRoutes_MissingUIReturns503(t *testing.T) {
	t.Parallel()

	// Empty FS — simulates a binary built without running "make ui-build".
	emptyFS := fstest.MapFS{}
	r := chi.NewRouter()
	registerUIRoutesWithFS(r, emptyFS)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
