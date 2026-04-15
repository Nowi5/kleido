package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"kleido/internal/logger"
	"kleido/internal/reqctx"
)

// RequestLogger returns a middleware that logs each request using the logger
// stored in the context (or slog.Default() if none). It records the request ID,
// method, path, status code, and elapsed time.
//
// It also injects the client IP and User-Agent into the context so that the
// service layer can include them in structured audit log events.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Inject the logger into the request context so handlers can use it.
			ctx := logger.WithContext(r.Context(), log)

			// Inject IP and User-Agent for structured audit logging in the service layer.
			ctx = reqctx.WithIP(ctx, r.RemoteAddr)
			ctx = reqctx.WithUserAgent(ctx, r.UserAgent())

			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			reqID := chimiddleware.GetReqID(ctx)
			log.InfoContext(ctx, "request",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("elapsed", time.Since(start)),
			)
		})
	}
}
