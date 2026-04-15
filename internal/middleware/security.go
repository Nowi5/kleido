// Package middleware provides HTTP middleware for the kleido API server.
package middleware

import "net/http"

// SecurityHeaders returns a middleware that sets defensive HTTP security headers
// on every response. When isProd is true, the Strict-Transport-Security header
// is also included and the CSP is tightened.
//
// In non-production mode the CSP allows 'unsafe-inline' scripts so that the
// Swagger UI (which ships an inline window.onload initialiser) renders correctly.
// Swagger is disabled in production, so the stricter policy applies there.
func SecurityHeaders(isProd bool) func(http.Handler) http.Handler {
	csp := "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"
	if isProd {
		csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("Content-Security-Policy", csp)
			if isProd {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
