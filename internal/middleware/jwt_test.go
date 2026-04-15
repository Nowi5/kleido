package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"kleido/internal/auth"
	"kleido/internal/middleware"
)

// fakeChecker implements middleware.SessionChecker for tests.
type fakeChecker struct {
	blocklisted map[string]bool
}

func (f *fakeChecker) IsBlocklisted(_ context.Context, jti string) (bool, error) {
	return f.blocklisted[jti], nil
}

func newTestSvc(t *testing.T) *auth.JWTService {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return auth.NewJWTService(priv, &priv.PublicKey, 15*time.Minute, 7)
}

func TestJWT_MissingHeader(t *testing.T) {
	t.Parallel()

	svc := newTestSvc(t)
	mw := middleware.JWT(svc, &fakeChecker{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestJWT_MalformedToken(t *testing.T) {
	t.Parallel()

	svc := newTestSvc(t)
	mw := middleware.JWT(svc, &fakeChecker{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestJWT_ValidToken_InjectsContext(t *testing.T) {
	t.Parallel()

	svc := newTestSvc(t)
	userID := uuid.New()
	tokenStr, jti, _, err := svc.IssueAccessToken(userID, "user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	checker := &fakeChecker{blocklisted: map[string]bool{jti: false}}

	var capturedUserID string
	h := middleware.JWT(svc, checker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		val, ok := r.Context().Value(middleware.CtxKeyUserID).(string)
		if ok {
			capturedUserID = val
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	if capturedUserID != userID.String() {
		t.Errorf("userID in context: want %q, got %q", userID.String(), capturedUserID)
	}
}

func TestJWT_BlocklistedJTI(t *testing.T) {
	t.Parallel()

	svc := newTestSvc(t)
	tokenStr, jti, _, err := svc.IssueAccessToken(uuid.New(), "user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	checker := &fakeChecker{blocklisted: map[string]bool{jti: true}}
	mw := middleware.JWT(svc, checker)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for blocklisted JTI, got %d", rr.Code)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	t.Parallel()

	h := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.CtxKeyRole, "user")
	h.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rr.Code)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	t.Parallel()

	h := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.CtxKeyRole, "admin")
	h.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}
