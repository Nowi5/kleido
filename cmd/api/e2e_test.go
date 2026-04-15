//go:build e2e

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"kleido/internal/auth"
	"kleido/internal/config"
	repopostgres "kleido/internal/repository/postgres"
	reporedis "kleido/internal/repository/redis"
	"kleido/internal/service"
	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ---------------------------------------------------------------------------
// captureMailer — captures the last reset URL for test inspection
// ---------------------------------------------------------------------------

type captureMailer struct {
	mu      sync.Mutex
	lastURL string
}

func (m *captureMailer) SendPasswordReset(_ context.Context, _, resetURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastURL = resetURL
	return nil
}

func (m *captureMailer) LastURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastURL
}

// ---------------------------------------------------------------------------
// e2eEnv — holds the full wired stack for one test function
// ---------------------------------------------------------------------------

type e2eEnv struct {
	srv    *httptest.Server
	mailer *captureMailer
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	// ── Postgres container ──────────────────────────────────────────────────
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForExposedPort("5432").WithStartupTimeout(120 * time.Second),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	pgHost, err := pgC.Host(ctx)
	if err != nil {
		t.Fatalf("postgres host: %v", err)
	}
	pgPort, err := pgC.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("postgres port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", pgHost, pgPort.Port())

	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
PollDB:
	for pollCtx.Err() == nil {
		exitCode, _, err := pgC.Exec(pollCtx, []string{"pg_isready", "-U", "test", "-d", "testdb"})
		if err == nil && exitCode == 0 {
			break PollDB
		}
		select {
		case <-pollCtx.Done():
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
	if pollCtx.Err() != nil {
		t.Fatalf("pg_isready: database not ready within 30s")
	}

	if err := repopostgres.RunMigrations(dsn, "../../migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	// ── Redis container ─────────────────────────────────────────────────────
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(120 * time.Second),
	}
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() { _ = redisC.Terminate(context.Background()) })

	redisHost, err := redisC.Host(ctx)
	if err != nil {
		t.Fatalf("redis host: %v", err)
	}
	redisPort, err := redisC.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("redis port: %v", err)
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})
	t.Cleanup(func() { _ = rdb.Close() })

	// ── Repositories ────────────────────────────────────────────────────────
	sessionRepo := reporedis.NewSessionRepo(rdb)
	cacheRepo := reporedis.NewCacheRepo(rdb)
	userRepo := repopostgres.NewTracedUserRepository(
		repopostgres.NewInstrumentedUserRepository(
			repopostgres.NewUserRepository(pool),
		),
	)

	// ── JWT service ─────────────────────────────────────────────────────────
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	jwtSvc := auth.NewJWTService(priv, &priv.PublicKey, 15*time.Minute, 7)

	// ── Application services ────────────────────────────────────────────────
	log := slog.Default()
	mailer := &captureMailer{}

	userSvc := service.NewUserService(userRepo, cacheRepo, log)
	authSvc := service.NewAuthService(userSvc, sessionRepo, jwtSvc, log, mailer, "http://localhost")

	// ── Config ──────────────────────────────────────────────────────────────
	cfg := &config.Config{
		App: config.AppConfig{
			Env:         "test",
			ServiceName: "e2e",
		},
	}

	// ── Router + test server ────────────────────────────────────────────────
	router := buildRouter(pool, rdb, cfg, log, jwtSvc, authSvc, userSvc, sessionRepo, sessionRepo)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &e2eEnv{srv: srv, mailer: mailer}
}

// ---------------------------------------------------------------------------
// e2eEnv helper methods
// ---------------------------------------------------------------------------

func (e *e2eEnv) post(t *testing.T, path, bodyJSON string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.srv.URL+path, bytes.NewBufferString(bodyJSON))
	if err != nil {
		t.Fatalf("build POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (e *e2eEnv) get(t *testing.T, path string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (e *e2eEnv) mustBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return b
}

func (e *e2eEnv) mustJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	b := e.mustBody(t, resp)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal JSON body: %v\nbody: %s", err, b)
	}
	return m
}

// refreshCookie returns the refresh_token cookie from a response, or nil.
func refreshCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

// resetTokenFromURL extracts the token query param from a reset URL like
// "http://localhost/reset-password?token=XXX".
func resetTokenFromURL(url string) string {
	parts := strings.SplitN(url, "token=", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// ---------------------------------------------------------------------------
// Test functions
// ---------------------------------------------------------------------------

func TestE2E_Register(t *testing.T) {
	e := newE2EEnv(t)

	// Case 1: valid registration → 201, body has "id" and "email"
	resp := e.post(t, "/api/v1/auth/register", `{"email":"alice@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusCreated {
		body := e.mustBody(t, resp)
		t.Errorf("expected 201 got %d; body: %s", resp.StatusCode, body)
	} else {
		m := e.mustJSON(t, resp)
		if _, ok := m["id"]; !ok {
			t.Errorf("response missing 'id' field; got %v", m)
		}
		if _, ok := m["email"]; !ok {
			t.Errorf("response missing 'email' field; got %v", m)
		}
	}

	// Case 2: duplicate email → 409
	resp = e.post(t, "/api/v1/auth/register", `{"email":"alice@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusConflict {
		body := e.mustBody(t, resp)
		t.Errorf("expected 409 got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Case 3: empty email/password → 400
	resp = e.post(t, "/api/v1/auth/register", `{"email":"","password":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		body := e.mustBody(t, resp)
		t.Errorf("expected 400 got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestE2E_Login(t *testing.T) {
	e := newE2EEnv(t)

	// Seed a user.
	resp := e.post(t, "/api/v1/auth/register", `{"email":"bob@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed user failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Case 1: correct credentials → 200, access_token in body, httpOnly refresh cookie
	resp = e.post(t, "/api/v1/auth/login", `{"email":"bob@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 got %d; body: %s", resp.StatusCode, body)
	} else {
		m := e.mustJSON(t, resp)
		if _, ok := m["access_token"]; !ok {
			t.Errorf("login response missing 'access_token'; got %v", m)
		}
		cookie := refreshCookie(resp)
		if cookie == nil {
			t.Errorf("expected refresh_token cookie, got none")
		} else if !cookie.HttpOnly {
			t.Errorf("refresh_token cookie should be HttpOnly")
		}
	}

	// Case 2: wrong password → 401
	resp = e.post(t, "/api/v1/auth/login", `{"email":"bob@e2e.test","password":"WrongPass!"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		body := e.mustBody(t, resp)
		t.Errorf("expected 401 got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Case 3: empty fields → 400
	resp = e.post(t, "/api/v1/auth/login", `{"email":"","password":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		body := e.mustBody(t, resp)
		t.Errorf("expected 400 got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestE2E_TokenLifecycle(t *testing.T) {
	e := newE2EEnv(t)

	// Register
	resp := e.post(t, "/api/v1/auth/register", `{"email":"carol@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login
	resp = e.post(t, "/api/v1/auth/login", `{"email":"carol@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", resp.StatusCode)
	}
	loginBody := e.mustJSON(t, resp)
	accessToken, _ := loginBody["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("no access_token in login response")
	}
	refreshCookieVal := refreshCookie(resp)
	if refreshCookieVal == nil {
		t.Fatalf("no refresh_token cookie after login")
	}

	// GET /users/me with access token → 200
	resp = e.get(t, "/api/v1/users/me", accessToken)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 from /users/me got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// POST /auth/refresh → 200, new access token, new cookie
	resp = e.post(t, "/api/v1/auth/refresh", `{}`, refreshCookieVal)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Fatalf("expected 200 from /auth/refresh got %d; body: %s", resp.StatusCode, body)
	}
	refreshBody := e.mustJSON(t, resp)
	newAccessToken, _ := refreshBody["access_token"].(string)
	if newAccessToken == "" {
		t.Fatalf("no access_token in refresh response")
	}
	newRefreshCookie := refreshCookie(resp)
	if newRefreshCookie == nil {
		t.Fatalf("no new refresh_token cookie after refresh")
	}

	// POST /auth/logout with new token + new cookie → 204
	// Logout requires both the Authorization header and the refresh cookie.
	logoutReq, err := http.NewRequest(http.MethodPost, e.srv.URL+"/api/v1/auth/logout", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("build logout request: %v", err)
	}
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutReq.Header.Set("Authorization", "Bearer "+newAccessToken)
	logoutReq.AddCookie(newRefreshCookie)
	resp, err = http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		body := e.mustBody(t, resp)
		t.Errorf("expected 204 from logout got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// GET /users/me with revoked token → 401 with "revoked" in body
	resp = e.get(t, "/api/v1/users/me", newAccessToken)
	if resp.StatusCode != http.StatusUnauthorized {
		body := e.mustBody(t, resp)
		t.Errorf("expected 401 after logout got %d; body: %s", resp.StatusCode, body)
	} else {
		body := e.mustBody(t, resp)
		if !strings.Contains(strings.ToLower(string(body)), "revoked") {
			t.Errorf("expected 'revoked' in body, got: %s", body)
		}
	}

	// GET /users/me with no token → 401
	resp = e.get(t, "/api/v1/users/me", "")
	if resp.StatusCode != http.StatusUnauthorized {
		body := e.mustBody(t, resp)
		t.Errorf("expected 401 with no token got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestE2E_AccessControl(t *testing.T) {
	e := newE2EEnv(t)

	// Register + login a regular user.
	resp := e.post(t, "/api/v1/auth/register", `{"email":"eve@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = e.post(t, "/api/v1/auth/login", `{"email":"eve@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", resp.StatusCode)
	}
	loginBody := e.mustJSON(t, resp)
	accessToken, _ := loginBody["access_token"].(string)
	if accessToken == "" {
		t.Fatalf("no access_token in login response")
	}

	// Case 1: non-admin accessing admin-only endpoint → 403
	resp = e.get(t, "/api/v1/users", accessToken)
	if resp.StatusCode != http.StatusForbidden {
		body := e.mustBody(t, resp)
		t.Errorf("expected 403 for non-admin listing users got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Case 2: malformed token → 401
	resp = e.get(t, "/api/v1/users/me", "not.a.valid.jwt")
	if resp.StatusCode != http.StatusUnauthorized {
		body := e.mustBody(t, resp)
		t.Errorf("expected 401 for malformed token got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestE2E_PasswordReset(t *testing.T) {
	e := newE2EEnv(t)

	// Register charlie.
	resp := e.post(t, "/api/v1/auth/register", `{"email":"charlie@e2e.test","password":"OldPass1!"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register charlie failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Confirm login works before reset.
	resp = e.post(t, "/api/v1/auth/login", `{"email":"charlie@e2e.test","password":"OldPass1!"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-reset login failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST forgot-password with known email → 200
	resp = e.post(t, "/api/v1/auth/forgot-password", `{"email":"charlie@e2e.test"}`)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 from forgot-password got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// POST forgot-password with unknown email → same 200 (enumeration-safe)
	resp = e.post(t, "/api/v1/auth/forgot-password", `{"email":"noone@e2e.test"}`)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 from forgot-password (unknown email) got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Extract reset token from captured mailer URL.
	lastURL := e.mailer.LastURL()
	if lastURL == "" {
		t.Fatalf("captureMailer has no URL — forgot-password did not trigger email")
	}
	resetToken := resetTokenFromURL(lastURL)
	if resetToken == "" {
		t.Fatalf("could not extract token from reset URL: %s", lastURL)
	}

	// POST reset-password → 200
	resetBody := fmt.Sprintf(`{"token":%q,"new_password":"NewPass2!"}`, resetToken)
	resp = e.post(t, "/api/v1/auth/reset-password", resetBody)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 from reset-password got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// POST reset-password with same token again → 400 (single-use)
	resp = e.post(t, "/api/v1/auth/reset-password", resetBody)
	if resp.StatusCode != http.StatusBadRequest {
		body := e.mustBody(t, resp)
		t.Errorf("expected 400 on reuse of reset token got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Login with new password → 200
	resp = e.post(t, "/api/v1/auth/login", `{"email":"charlie@e2e.test","password":"NewPass2!"}`)
	if resp.StatusCode != http.StatusOK {
		body := e.mustBody(t, resp)
		t.Errorf("expected 200 login with new password got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Login with old password → 401
	resp = e.post(t, "/api/v1/auth/login", `{"email":"charlie@e2e.test","password":"OldPass1!"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		body := e.mustBody(t, resp)
		t.Errorf("expected 401 login with old password got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestE2E_BruteForce(t *testing.T) {
	e := newE2EEnv(t)

	// Register dave.
	resp := e.post(t, "/api/v1/auth/register", `{"email":"dave@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register dave failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fire 10 wrong-password attempts; each should be 401 or possibly 429 near the threshold.
	for i := 1; i <= 10; i++ {
		resp = e.post(t, "/api/v1/auth/login", `{"email":"dave@e2e.test","password":"WrongPass!"}`)
		code := resp.StatusCode
		resp.Body.Close()
		if code != http.StatusUnauthorized && code != http.StatusTooManyRequests {
			t.Errorf("attempt %d: expected 401 or 429 got %d", i, code)
		}
	}

	// 11th request — even with correct password — must be 429 (locked out).
	resp = e.post(t, "/api/v1/auth/login", `{"email":"dave@e2e.test","password":"Secret123!"}`)
	if resp.StatusCode != http.StatusTooManyRequests {
		body := e.mustBody(t, resp)
		t.Errorf("expected 429 on 11th attempt got %d; body: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}
