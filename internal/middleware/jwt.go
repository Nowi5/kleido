package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/nowi5/kleido/internal/auth"
	"github.com/nowi5/kleido/pkg/apperror"
)

// ctxKey is the unexported type used for context keys in this package.
// Using a dedicated type prevents collision with other packages.
type ctxKey string

// Context keys injected by the JWT middleware.
const (
	CtxKeyUserID ctxKey = "userID" // string UUID of the authenticated user
	CtxKeyRole   ctxKey = "role"   // string role of the authenticated user
	CtxKeyJTI    ctxKey = "jti"    // string JTI of the access token
)

// SessionChecker is the narrow interface the JWT middleware needs from the
// session repository. It prevents importing the full repository package.
type SessionChecker interface {
	IsBlocklisted(ctx context.Context, jti string) (bool, error)
}

// JWT validates the Bearer token in the Authorization header and injects
// the claims (userID, role, jti) into the request context.
//
// Order of checks:
//  1. Extract Bearer token — 401 if missing
//  2. Parse and verify signature — 401 if invalid
//  3. Check JTI blocklist — 401 if revoked
//  4. Inject userID, role, jti into context
func JWT(svc *auth.JWTService, sessions SessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				apperror.WriteError(w, apperror.Unauthorized("missing or invalid authorization header"))
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := svc.Verify(tokenStr)
			if err != nil {
				apperror.WriteError(w, apperror.Unauthorized("invalid token"))
				return
			}

			blocked, err := sessions.IsBlocklisted(r.Context(), claims.ID)
			if err != nil { //nolint:staticcheck // intentional fail-open: Redis down → allow through
				// Fail open — log in production via structured logger when available.
				_ = err
			}
			if blocked {
				apperror.WriteError(w, apperror.Unauthorized("token has been revoked"))
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, CtxKeyRole, claims.Role)
			ctx = context.WithValue(ctx, CtxKeyJTI, claims.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns 403 Forbidden if the role stored in context does not
// match the required role. Must be used after the JWT middleware.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxRole, ok := r.Context().Value(CtxKeyRole).(string)
			if !ok {
				ctxRole = ""
			}
			if ctxRole != role {
				apperror.WriteError(w, apperror.Forbidden("insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
