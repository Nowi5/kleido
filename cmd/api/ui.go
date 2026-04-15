package main

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	webstatic "kleido/web"
)

//go:generate make ui-build

// registerUIRoutes wires the compiled React SPA and static asset serving into
// the router. It MUST be called AFTER all /api/*, /admin/*, /metrics, /healthz,
// /readyz, and /swagger/* routes — the /* catch-all must not shadow them.
func registerUIRoutes(r chi.Router) {
	distFS, err := fs.Sub(webstatic.Files, "dist")
	if err != nil {
		// This should never happen at runtime if the binary was built with
		// "make build" (which runs ui-build first). Panic with a clear message.
		panic("embed: web/dist sub-FS unavailable — run 'make ui-build' before 'go build': " + err.Error())
	}
	registerUIRoutesWithFS(r, distFS)
}

// registerUIRoutesWithFS is the testable core — accepts any fs.FS so tests can
// inject a fake filesystem without requiring a real Vite build.
func registerUIRoutesWithFS(r chi.Router, distFS fs.FS) {
	fileServer := http.FileServer(http.FS(distFS))

	// 1. Hashed assets — immutable cache.
	//    Vite embeds a content hash in every asset filename (e.g. main-Bk3F9cA2.js),
	//    so these files are safe to cache for a full year.
	r.Get("/assets/*", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	})

	// 2. Well-known root files — moderate TTL.
	for _, f := range []string{"/favicon.ico", "/robots.txt", "/manifest.json"} {
		path := f
		r.Get(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=86400")
			fileServer.ServeHTTP(w, r)
		})
	}

	// 3. SPA catch-all.
	//    index.html gets Cache-Control: no-store — browsers must always fetch
	//    the latest index.html so they pick up new asset hashes on deploy.
	//    Unknown paths (deep routes) also return index.html for client-side routing.
	r.Get("/*", spaHandler(distFS))
}

// spaHandler returns an http.HandlerFunc that serves static files from distFS
// or falls back to index.html for SPA client-side routing.
func spaHandler(distFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Always serve index.html with no-store so the browser fetches the
		// latest version (which contains current asset hashes).
		if path == "index.html" {
			serveIndex(w, distFS)
			return
		}

		// Serve existing static files directly.
		if _, err := fs.Stat(distFS, path); err == nil {
			// Non-hashed files (images, fonts without hash in name) get a short TTL.
			if !strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}
			http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
			return
		}

		// Unknown path — serve index.html so the React router handles it.
		serveIndex(w, distFS)
	}
}

func serveIndex(w http.ResponseWriter, distFS fs.FS) {
	content, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		http.Error(w, "UI not built — run 'make ui-build'", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(content)
}
