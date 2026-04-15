package handler_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kleido/internal/auth"
	"kleido/internal/handler"
	"kleido/internal/model"
	"kleido/internal/service"
	"kleido/pkg/apperror"

	"github.com/google/uuid"
)

// --- Mocks ---

type mockAuthSvc struct {
	loginPair   *service.TokenPair
	loginErr    error
	user        *model.User
	registerErr error
}

func (m *mockAuthSvc) Register(_ context.Context, email, _ string) (*model.User, error) {
	if m.registerErr != nil {
		return nil, m.registerErr
	}
	if m.user != nil {
		return m.user, nil
	}
	return &model.User{ID: uuid.New(), Email: email, Role: "user", IsActive: true}, nil
}

func (m *mockAuthSvc) Login(_ context.Context, _, _ string) (*service.TokenPair, error) {
	return m.loginPair, m.loginErr
}

func (m *mockAuthSvc) Refresh(_ context.Context, _ string) (*service.TokenPair, error) {
	return m.loginPair, m.loginErr
}

func (m *mockAuthSvc) Logout(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockAuthSvc) ForgotPassword(_ context.Context, _ string) error {
	return nil
}

func (m *mockAuthSvc) ResetPassword(_ context.Context, _, _ string) error {
	return m.loginErr
}

// --- Helpers ---

func newTestJWTSvcH(t *testing.T) *auth.JWTService {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return auth.NewJWTService(priv, &priv.PublicKey, 15*time.Minute, 7)
}

func newAuthHandler(svc service.AuthService) *handler.AuthHandler {
	jwtSvc := func() *auth.JWTService {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		return auth.NewJWTService(priv, &priv.PublicKey, 15*time.Minute, 7)
	}()
	return handler.NewAuthHandler(svc, jwtSvc, false, true)
}

// --- Tests ---

func TestLogin_ValidCredentials_ReturnsAccessTokenAndCookie(t *testing.T) {
	t.Parallel()

	jwtSvc := newTestJWTSvcH(t)
	pair := &service.TokenPair{
		AccessToken:     "test-access-token",
		JTI:             "test-jti",
		ExpiresAt:       time.Now().Add(15 * time.Minute),
		RawRefreshToken: "raw-refresh-token",
	}
	svc := &mockAuthSvc{loginPair: pair}
	h := handler.NewAuthHandler(svc, jwtSvc, false, true)

	body := `{"email":"a@b.com","password":"Password1!"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := resp["access_token"]; !ok {
		t.Error("response must contain access_token")
	}
	if _, ok := resp["refresh_token"]; ok {
		t.Error("refresh_token must NOT appear in the response body")
	}

	cookies := rr.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			found = true
			if !c.HttpOnly {
				t.Error("refresh_token cookie must be HttpOnly")
			}
			break
		}
	}
	if !found {
		t.Error("refresh_token cookie must be set")
	}
}

func TestLogin_MissingFields_Returns400(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{}
	h := newAuthHandler(svc)

	body := `{"email":"","password":""}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestRegister_Returns201(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{}
	h := newAuthHandler(svc)

	body := `{"email":"new@example.com","password":"Password1!"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRegister_DuplicateEmail_Returns409(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{registerErr: apperror.Conflict("email already registered")}
	h := newAuthHandler(svc)

	body := `{"email":"dup@example.com","password":"Password1!"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	h.Register(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("want 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRegister_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{}
	h := newAuthHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestRefresh_MissingCookie_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{loginErr: apperror.Unauthorized("invalid token")}
	h := newAuthHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/refresh", nil)
	h.Refresh(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestRefresh_WithValidCookie_ReturnsNewTokenAndCookie(t *testing.T) {
	t.Parallel()

	jwtSvc := newTestJWTSvcH(t)
	pair := &service.TokenPair{
		AccessToken:     "new-access-token",
		JTI:             "new-jti",
		ExpiresAt:       time.Now().Add(15 * time.Minute),
		RawRefreshToken: "new-refresh-token",
	}
	svc := &mockAuthSvc{loginPair: pair}
	h := handler.NewAuthHandler(svc, jwtSvc, false, true)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-refresh-token"})
	h.Refresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["access_token"]; !ok {
		t.Error("response must contain access_token")
	}

	// New refresh cookie must be set and be HttpOnly.
	var found bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" {
			found = true
			if !c.HttpOnly {
				t.Error("new refresh_token cookie must be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("new refresh_token cookie must be set after refresh")
	}
}

func TestLogout_Returns204AndClearsCookie(t *testing.T) {
	t.Parallel()

	svc := &mockAuthSvc{}
	h := newAuthHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "some-refresh-token"})
	h.Logout(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// The response must set a cookie that clears refresh_token (MaxAge < 0).
	var found bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" {
			found = true
			if c.MaxAge >= 0 {
				t.Errorf("logout cookie MaxAge must be negative to clear it, got %d", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("Logout must set a clearing refresh_token cookie")
	}
}
