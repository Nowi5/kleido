package service_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"kleido/internal/auth"
	"kleido/internal/logger"
	"kleido/internal/model"
	"kleido/internal/service"
	"kleido/pkg/apperror"
	"golang.org/x/crypto/bcrypt"
)

// --- Mocks ---

type mockUserSvc struct {
	byEmail        map[string]*model.User
	byID           map[uuid.UUID]*model.User
	updatePWCalled bool
	updatePWErr    error
}

func (m *mockUserSvc) GetByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

func (m *mockUserSvc) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

func (m *mockUserSvc) Create(_ context.Context, email, _, role string) (*model.User, error) {
	u := &model.User{ID: uuid.New(), Email: email, Role: role, IsActive: true}
	return u, nil
}

func (m *mockUserSvc) List(_ context.Context, _, _ int) ([]*model.UserResponse, int64, error) {
	return nil, 0, nil
}

func (m *mockUserSvc) Update(_ context.Context, _ uuid.UUID, _ *model.UpdateUserRequest, _ string) (*model.User, error) {
	return nil, nil //nolint:nilnil
}

func (m *mockUserSvc) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockUserSvc) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	m.updatePWCalled = true
	return m.updatePWErr
}

// mockSessionRepo satisfies repository.SessionRepository for unit tests.
type mockSessionRepo struct {
	stored             map[string]string // token → userID (raw, not hashed)
	blocklist          map[string]bool
	blocklistCallCount int
	revokeCallCount    int
	rotateOldToken     string
	rotateNewToken     string
	rotateUserID       string
	rotateTTL          time.Duration
	rotateCallCount    int

	// lockout state
	failureCounts       map[string]int64
	clearFailuresCalled bool
	isLockedOutReturn   bool
	isLockedOutErr      error

	// password reset
	resetTokens    map[string]string // token → userID
	consumeResetFn func(token string) (string, error)
}

func (m *mockSessionRepo) StoreRefreshToken(_ context.Context, token, userID string, _ time.Duration) error {
	if m.stored == nil {
		m.stored = map[string]string{}
	}
	m.stored[token] = userID
	return nil
}

func (m *mockSessionRepo) ValidateRefreshToken(_ context.Context, token string) (string, error) {
	id, ok := m.stored[token]
	if !ok {
		return "", apperror.Unauthorized("invalid token")
	}
	return id, nil
}

func (m *mockSessionRepo) RevokeRefreshToken(_ context.Context, _ string) error {
	m.revokeCallCount++
	return nil
}

func (m *mockSessionRepo) BlocklistJTI(_ context.Context, _ string, _ time.Duration) error {
	m.blocklistCallCount++
	return nil
}

func (m *mockSessionRepo) IsBlocklisted(_ context.Context, jti string) (bool, error) {
	return m.blocklist[jti], nil
}

func (m *mockSessionRepo) RateLimitAllow(_ context.Context, _ string, limit int64, _ time.Duration) (bool, int64, time.Time, error) {
	return true, limit, time.Now().Add(time.Minute), nil
}

func (m *mockSessionRepo) RotateRefreshToken(_ context.Context, oldToken, newToken, userID string, ttl time.Duration) error {
	m.rotateCallCount++
	m.rotateOldToken = oldToken
	m.rotateNewToken = newToken
	m.rotateUserID = userID
	m.rotateTTL = ttl
	if m.stored == nil {
		m.stored = map[string]string{}
	}
	delete(m.stored, oldToken)
	m.stored[newToken] = userID
	return nil
}

func (m *mockSessionRepo) IncrLoginFailure(_ context.Context, email string, _ time.Duration) (int64, error) {
	if m.failureCounts == nil {
		m.failureCounts = map[string]int64{}
	}
	m.failureCounts[email]++
	return m.failureCounts[email], nil
}

func (m *mockSessionRepo) GetLoginFailures(_ context.Context, email string) (int64, error) {
	if m.failureCounts == nil {
		return 0, nil
	}
	return m.failureCounts[email], nil
}

func (m *mockSessionRepo) ClearLoginFailures(_ context.Context, _ string) error {
	m.clearFailuresCalled = true
	return nil
}

func (m *mockSessionRepo) IsLockedOut(_ context.Context, _ string) (bool, error) {
	return m.isLockedOutReturn, m.isLockedOutErr
}

func (m *mockSessionRepo) RateLimitAllowUser(_ context.Context, _, _ string, limit int64, _ time.Duration) (bool, int64, time.Time, error) {
	return true, limit, time.Now().Add(time.Minute), nil
}

func (m *mockSessionRepo) StorePasswordResetToken(_ context.Context, token, userID string) error {
	if m.resetTokens == nil {
		m.resetTokens = map[string]string{}
	}
	m.resetTokens[token] = userID
	return nil
}

func (m *mockSessionRepo) ConsumePasswordResetToken(_ context.Context, token string) (string, error) {
	if m.consumeResetFn != nil {
		return m.consumeResetFn(token)
	}
	if m.resetTokens == nil {
		return "", errors.New("reset token not found or already used")
	}
	uid, ok := m.resetTokens[token]
	if !ok {
		return "", errors.New("reset token not found or already used")
	}
	delete(m.resetTokens, token)
	return uid, nil
}

// --- Helpers ---

func newTestJWTSvc(t *testing.T) *auth.JWTService {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return auth.NewJWTService(priv, &priv.PublicKey, 15*time.Minute, 7)
}

func hashPW(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 4)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

func newSvc(t *testing.T, userSvc service.UserService, sessions *mockSessionRepo) service.AuthService {
	t.Helper()
	jwtSvc := newTestJWTSvc(t)
	return service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "http://localhost:8080")
}

// --- Variant mocks for error injection ---

type mockSessionRepoStoreErr struct {
	mockSessionRepo
	err error
}

func (m *mockSessionRepoStoreErr) StoreRefreshToken(_ context.Context, _, _ string, _ time.Duration) error {
	return m.err
}

type mockSessionRepoBlocklistErr struct {
	mockSessionRepo
	err error
}

func (m *mockSessionRepoBlocklistErr) BlocklistJTI(_ context.Context, _ string, _ time.Duration) error {
	return m.err
}

type mockSessionRepoRevokeErr struct {
	mockSessionRepo
	err error
}

func (m *mockSessionRepoRevokeErr) RevokeRefreshToken(_ context.Context, _ string) error {
	return m.err
}

type mockUserSvcCreateErr struct {
	mockUserSvc
	err error
}

func (m *mockUserSvcCreateErr) Create(_ context.Context, _, _, _ string) (*model.User, error) {
	return nil, m.err
}

// --- Register tests ---

func TestRegister_Success(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byEmail: map[string]*model.User{}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	user, err := svc.Register(context.Background(), "new@example.com", "Password1!")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("email: want %q, got %q", "new@example.com", user.Email)
	}
	if user.Role != "user" {
		t.Errorf("role: want %q, got %q", "user", user.Role)
	}
}

func TestRegister_UserSvcError(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	errUserSvc := &mockUserSvcCreateErr{err: apperror.Conflict("email already registered")}
	svc := service.NewAuthService(errUserSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Register(context.Background(), "dup@example.com", "Password1!")
	if err == nil {
		t.Fatal("expected error from Register")
	}
}

// --- Logout tests ---

func TestLogout_BlocklistError(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepoBlocklistErr{err: errors.New("redis down")}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	if err := svc.Logout(context.Background(), "some-jti", "some-refresh-token"); err == nil {
		t.Fatal("expected error when BlocklistJTI fails")
	}
}

func TestLogout_RevokeError(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepoRevokeErr{err: errors.New("redis down")}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	if err := svc.Logout(context.Background(), "some-jti", "some-refresh-token"); err == nil {
		t.Fatal("expected error when RevokeRefreshToken fails")
	}
}

// --- Refresh tests ---

func TestRefresh_UserNotFound(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	rawRefresh := "some-refresh-token"
	sessions := &mockSessionRepo{stored: map[string]string{rawRefresh: userID.String()}}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byID: map[uuid.UUID]*model.User{}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Refresh(context.Background(), rawRefresh)
	if err == nil {
		t.Fatal("expected error when user not found during refresh")
	}
	if !apperror.IsUnauthorized(err) {
		t.Errorf("expected Unauthorized, got: %v", err)
	}
}

// --- Login tests ---

func TestLogin_StoreRefreshError(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	pw := "Password1!"
	user := &model.User{ID: userID, Email: "a@b.com", PasswordHash: hashPW(t, pw), Role: "user", IsActive: true}

	sessions := &mockSessionRepoStoreErr{err: errors.New("redis down")}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"a@b.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Login(context.Background(), "a@b.com", pw)
	if err == nil {
		t.Fatal("expected error when StoreRefreshToken fails")
	}
}

func TestLogin_ValidCredentials(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	pw := "Password1!"
	user := &model.User{ID: userID, Email: "test@example.com", PasswordHash: hashPW(t, pw), Role: "user", IsActive: true}

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"test@example.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	pair, err := svc.Login(context.Background(), "test@example.com", pw)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("AccessToken must not be empty")
	}
	if pair.RawRefreshToken == "" {
		t.Error("RawRefreshToken must not be empty")
	}
	if !sessions.clearFailuresCalled {
		t.Error("ClearLoginFailures must be called on successful login")
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byEmail: map[string]*model.User{}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Login(context.Background(), "nobody@example.com", "anything")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
	if !apperror.IsUnauthorized(err) {
		t.Errorf("expected Unauthorized, got: %v", err)
	}
	var ae *apperror.AppError
	if errors.As(err, &ae) && ae.Message == "user not found" {
		t.Error("error message must not reveal that the user doesn't exist")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "a@b.com", PasswordHash: hashPW(t, "correctpw"), Role: "user", IsActive: true}

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"a@b.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Login(context.Background(), "a@b.com", "wrongpw")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !apperror.IsUnauthorized(err) {
		t.Errorf("expected Unauthorized, got: %v", err)
	}
}

// --- Lockout tests ---

func TestLogin_LockedAccount_Returns429(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{isLockedOutReturn: true}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Login(context.Background(), "locked@example.com", "anything")
	if err == nil {
		t.Fatal("expected error for locked account")
	}
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != 429 {
		t.Errorf("expected 429, got: %v", err)
	}
}

func TestLogin_FailureIncrementsClearOnSuccess(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	pw := "Password1!"
	user := &model.User{ID: userID, Email: "a@b.com", PasswordHash: hashPW(t, pw), Role: "user", IsActive: true}

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"a@b.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	// Two wrong password attempts.
	for i := 0; i < 2; i++ {
		_, _ = svc.Login(context.Background(), "a@b.com", "wrong")
	}
	if sessions.failureCounts["a@b.com"] != 2 {
		t.Errorf("want 2 failure increments, got %d", sessions.failureCounts["a@b.com"])
	}

	// Correct password — counter must be cleared.
	_, err := svc.Login(context.Background(), "a@b.com", pw)
	if err != nil {
		t.Fatalf("Login with correct password: %v", err)
	}
	if !sessions.clearFailuresCalled {
		t.Error("ClearLoginFailures must be called after successful login")
	}
}

// --- Logout tests ---

func TestLogout_BlocklistsAndRevokes(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	if err := svc.Logout(context.Background(), "some-jti", "some-refresh-token"); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if sessions.blocklistCallCount != 1 {
		t.Errorf("BlocklistJTI called %d times, want 1", sessions.blocklistCallCount)
	}
	if sessions.revokeCallCount != 1 {
		t.Errorf("RevokeRefreshToken called %d times, want 1", sessions.revokeCallCount)
	}
}

// --- Refresh tests ---

func TestRefresh_ValidToken(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "u@e.com", Role: "user", IsActive: true}

	rawRefresh := "raw-refresh-abc"
	sessions := &mockSessionRepo{stored: map[string]string{rawRefresh: userID.String()}}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byID: map[uuid.UUID]*model.User{userID: user}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	pair, err := svc.Refresh(context.Background(), rawRefresh)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("new AccessToken must not be empty")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{stored: map[string]string{}}
	jwtSvc := newTestJWTSvc(t)
	svc := service.NewAuthService(&mockUserSvc{}, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Refresh(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !apperror.IsUnauthorized(err) {
		t.Errorf("expected Unauthorized, got: %v", err)
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "u@e.com", Role: "user", IsActive: true}

	oldRefresh := "old-refresh-token"
	sessions := &mockSessionRepo{stored: map[string]string{oldRefresh: userID.String()}}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byID: map[uuid.UUID]*model.User{userID: user}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	pair, err := svc.Refresh(context.Background(), oldRefresh)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if sessions.rotateCallCount != 1 {
		t.Errorf("RotateRefreshToken call count: want 1, got %d", sessions.rotateCallCount)
	}
	if sessions.rotateOldToken != oldRefresh {
		t.Errorf("rotated old token: want %q, got %q", oldRefresh, sessions.rotateOldToken)
	}
	if pair.RawRefreshToken == oldRefresh {
		t.Error("RawRefreshToken must be a new value after rotation")
	}
	if pair.RawRefreshToken != sessions.rotateNewToken {
		t.Errorf("RawRefreshToken mismatch: pair=%q, rotateNewToken=%q", pair.RawRefreshToken, sessions.rotateNewToken)
	}
	_, validateErr := sessions.ValidateRefreshToken(context.Background(), oldRefresh)
	if validateErr == nil {
		t.Error("old refresh token must be invalid after rotation")
	}
}

// --- Audit log tests ---

func TestLogin_AuditEvents_Failure(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLog := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := logger.WithContext(context.Background(), testLog)

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "user@example.com", PasswordHash: hashPW(t, "correct"), Role: "user"}
	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"user@example.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, _ = svc.Login(ctx, "user@example.com", "wrongpassword")

	var foundFailure bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record["event_type"] == service.EventLoginFailure {
			foundFailure = true
			masked, _ := record["email_masked"].(string)
			if masked == "" {
				t.Error("email_masked must be present in failure audit event")
			}
			if strings.Contains(masked, "example.com") {
				// email_masked should mask the domain too
				// "user@example.com" → "u***@e***.com" — "example.com" should not appear verbatim
				t.Errorf("email_masked must not contain full domain, got %q", masked)
			}
		}
	}
	if !foundFailure {
		t.Errorf("expected auth.login.failure audit event in log output, got:\n%s", buf.String())
	}
}

func TestLogin_AuditEvents_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLog := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := logger.WithContext(context.Background(), testLog)

	userID := uuid.New()
	pw := "Password1!"
	user := &model.User{ID: userID, Email: "user@example.com", PasswordHash: hashPW(t, pw), Role: "user"}
	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"user@example.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	_, err := svc.Login(ctx, "user@example.com", pw)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	var foundSuccess bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record["event_type"] == service.EventLoginSuccess {
			foundSuccess = true
			if record["user_id"] == "" {
				t.Error("user_id must be present in success audit event")
			}
		}
	}
	if !foundSuccess {
		t.Errorf("expected auth.login.success audit event, got:\n%s", buf.String())
	}
}

// --- Password reset tests ---

func TestForgotPassword_KnownEmail_StoresToken(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "user@example.com", Role: "user"}
	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byEmail: map[string]*model.User{"user@example.com": user}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, &service.StubEmailSender{}, "http://localhost:8080")

	err := svc.ForgotPassword(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	if len(sessions.resetTokens) == 0 {
		t.Error("StorePasswordResetToken must be called for known email")
	}
}

func TestForgotPassword_UnknownEmail_ReturnsNil(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{byEmail: map[string]*model.User{}}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	// Must return nil — never 404 — to prevent enumeration.
	if err := svc.ForgotPassword(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("ForgotPassword for unknown email must return nil, got: %v", err)
	}
	if len(sessions.resetTokens) != 0 {
		t.Error("StorePasswordResetToken must NOT be called for unknown email")
	}
}

func TestResetPassword_ValidToken(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &model.User{ID: userID, Email: "user@example.com", Role: "user"}
	rawToken := "valid-reset-token"

	sessions := &mockSessionRepo{
		resetTokens: map[string]string{rawToken: userID.String()},
	}
	jwtSvc := newTestJWTSvc(t)
	userSvc := &mockUserSvc{
		byEmail: map[string]*model.User{"user@example.com": user},
		byID:    map[uuid.UUID]*model.User{userID: user},
	}
	svc := service.NewAuthService(userSvc, sessions, jwtSvc, nil, nil, "")

	if err := svc.ResetPassword(context.Background(), rawToken, "NewPassword1!"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if !userSvc.updatePWCalled {
		t.Error("UpdatePassword must be called after consuming reset token")
	}
	// Token must be consumed (deleted from the map).
	if len(sessions.resetTokens) != 0 {
		t.Error("reset token must be deleted after consumption")
	}
}

func TestResetPassword_ExpiredToken_Returns400(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{resetTokens: map[string]string{}} // empty — no valid tokens
	svc := newSvc(t, &mockUserSvc{}, sessions)

	err := svc.ResetPassword(context.Background(), "expired-or-used-token", "NewPassword1!")
	if err == nil {
		t.Fatal("expected error for expired/used token")
	}
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != 400 {
		t.Errorf("expected 400 BadRequest, got: %v", err)
	}
}

func TestResetPassword_ShortPassword_Returns400(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionRepo{}
	svc := newSvc(t, &mockUserSvc{}, sessions)

	err := svc.ResetPassword(context.Background(), "any-token", "short")
	if err == nil {
		t.Fatal("expected error for short password")
	}
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != 400 {
		t.Errorf("expected 400 BadRequest, got: %v", err)
	}
}

func TestResetPassword_TokenConsumedBeforePasswordUpdate(t *testing.T) {
	t.Parallel()

	// Track call order.
	var callOrder []string
	userID := uuid.New()
	user := &model.User{ID: userID, Email: "u@e.com", Role: "user"}

	sessions := &mockSessionRepo{}
	sessions.consumeResetFn = func(_ string) (string, error) {
		callOrder = append(callOrder, "consume")
		return userID.String(), nil
	}

	// Use a custom userSvc that tracks order.
	orderedUserSvc := &mockUserSvcWithOrder{
		mockUserSvc: mockUserSvc{byID: map[uuid.UUID]*model.User{userID: user}},
		onUpdatePW:  func() { callOrder = append(callOrder, "update_password") },
	}
	jwtSvc := newTestJWTSvc(t)
	svc := service.NewAuthService(orderedUserSvc, sessions, jwtSvc, nil, nil, "")

	if err := svc.ResetPassword(context.Background(), "some-token", "NewPassword1!"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if len(callOrder) != 2 || callOrder[0] != "consume" || callOrder[1] != "update_password" {
		t.Errorf("expected [consume, update_password], got %v", callOrder)
	}
}

// mockUserSvcWithOrder extends mockUserSvc to track UpdatePassword call order.
type mockUserSvcWithOrder struct {
	mockUserSvc
	onUpdatePW func()
}

func (m *mockUserSvcWithOrder) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	if m.onUpdatePW != nil {
		m.onUpdatePW()
	}
	return nil
}
