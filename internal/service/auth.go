package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"kleido/internal/auth"
	"kleido/internal/logger"
	"kleido/internal/model"
	"kleido/internal/repository"
	"kleido/internal/reqctx"
	"kleido/pkg/apperror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/crypto/bcrypt"
)

var authTracer = otel.Tracer("kleido/service")

const loginLockoutWindow = 15 * time.Minute

type authService struct {
	userSvc    UserService
	sessions   repository.SessionRepository
	jwt        *auth.JWTService
	log        *slog.Logger
	mailer     EmailSender
	appBaseURL string
}

// NewAuthService creates an AuthService.
// mailer may be nil — if nil, password reset emails are silently skipped.
// appBaseURL is used to construct the reset link (e.g. "http://localhost:8080").
func NewAuthService(
	userSvc UserService,
	sessions repository.SessionRepository,
	jwt *auth.JWTService,
	log *slog.Logger,
	mailer EmailSender,
	appBaseURL string,
) AuthService {
	return &authService{
		userSvc:    userSvc,
		sessions:   sessions,
		jwt:        jwt,
		log:        log,
		mailer:     mailer,
		appBaseURL: appBaseURL,
	}
}

// Register creates a new user account with the "user" role.
func (s *authService) Register(ctx context.Context, email, password string) (*model.User, error) {
	ctx, span := authTracer.Start(ctx, "service/Register")
	defer span.End()

	user, err := s.userSvc.Create(ctx, email, password, "user") //nolint:wrapcheck // apperror from service layer; wrapping loses type
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return user, nil
}

// Login authenticates a user and returns a TokenPair on success.
// It checks the brute-force lockout before any DB query to prevent timing attacks.
// On successful login the failure counter is cleared. On failure it is incremented.
func (s *authService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	ctx, span := authTracer.Start(ctx, "service/Login")
	defer span.End()

	log := logger.FromContext(ctx)

	// 1. Check lockout FIRST — before any DB query — to prevent timing attacks
	//    that could reveal whether an email exists.
	locked, lockErr := s.sessions.IsLockedOut(ctx, email)
	if lockErr != nil {
		// Log but continue — never deny service due to a lockout check failure.
		log.WarnContext(ctx, "lockout check failed", slog.Any("error", lockErr))
	}
	if locked {
		s.auditLog(ctx, EventLoginLocked, email, "", nil)
		appErr := apperror.New(http.StatusTooManyRequests,
			"account temporarily locked due to too many failed attempts — try again in 15 minutes", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	// 2. Fetch user — return a generic error on not-found (account enumeration prevention).
	user, err := s.userSvc.GetByEmail(ctx, email)
	if err != nil {
		// Do not increment the failure counter for non-existent emails — this
		// would allow an attacker to exhaust counters for arbitrary addresses.
		s.auditLog(ctx, EventLoginFailure, email, "", nil)
		appErr := apperror.Unauthorized("invalid credentials")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	// 3. Verify password.
	if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); cmpErr != nil {
		// Increment the failure counter only for confirmed accounts.
		count, incrErr := s.sessions.IncrLoginFailure(ctx, email, loginLockoutWindow)
		if incrErr != nil {
			log.WarnContext(ctx, "failed to increment login failure counter", slog.Any("error", incrErr))
		}
		s.auditLog(ctx, EventLoginFailure, email, user.ID.String(), nil)
		if count >= 10 {
			s.auditLog(ctx, EventLoginLocked, email, user.ID.String(), nil)
		}
		appErr := apperror.Unauthorized("invalid credentials")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	// 4. Clear the failure counter on successful authentication.
	if err := s.sessions.ClearLoginFailures(ctx, email); err != nil {
		// Non-fatal — proceed with login even if clearing fails.
		log.WarnContext(ctx, "failed to clear login failure counter", slog.Any("error", err))
	}

	// 5. Issue tokens.
	tokenStr, jti, expiresAt, err := s.jwt.IssueAccessToken(user.ID, user.Role)
	if err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	rawRefresh, err := s.jwt.IssueRefreshToken()
	if err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	if err := s.sessions.StoreRefreshToken(ctx, rawRefresh, user.ID.String(), s.jwt.RefreshTTL()); err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	s.auditLog(ctx, EventLoginSuccess, email, user.ID.String(), nil)
	log.InfoContext(ctx, "user logged in", slog.String("user_id", user.ID.String()))
	span.SetStatus(codes.Ok, "")

	return &TokenPair{
		AccessToken:     tokenStr,
		JTI:             jti,
		ExpiresAt:       expiresAt,
		RawRefreshToken: rawRefresh,
	}, nil
}

// Refresh issues a new access token and rotates the refresh token.
// The old refresh token is atomically revoked and replaced with a new one —
// using an old token after rotation returns Unauthorized.
func (s *authService) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	ctx, span := authTracer.Start(ctx, "service/Refresh")
	defer span.End()

	userIDStr, err := s.sessions.ValidateRefreshToken(ctx, rawRefreshToken)
	if err != nil {
		appErr := apperror.Unauthorized("invalid or expired refresh token")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	user, err := s.userSvc.GetByID(ctx, userID)
	if err != nil {
		appErr := apperror.Unauthorized("user not found")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	tokenStr, jti, expiresAt, err := s.jwt.IssueAccessToken(user.ID, user.Role)
	if err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	newRawRefresh, err := s.jwt.IssueRefreshToken()
	if err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	if err := s.sessions.RotateRefreshToken(ctx, rawRefreshToken, newRawRefresh, user.ID.String(), s.jwt.RefreshTTL()); err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	s.auditLog(ctx, EventTokenRefresh, "", user.ID.String(), nil)
	span.SetStatus(codes.Ok, "")
	return &TokenPair{
		AccessToken:     tokenStr,
		JTI:             jti,
		ExpiresAt:       expiresAt,
		RawRefreshToken: newRawRefresh,
	}, nil
}

// Logout blocklists the access token's JTI and revokes the refresh token.
func (s *authService) Logout(ctx context.Context, jti, rawRefreshToken string) error {
	ctx, span := authTracer.Start(ctx, "service/Logout")
	defer span.End()

	log := logger.FromContext(ctx)

	if err := s.sessions.BlocklistJTI(ctx, jti, s.jwt.RefreshTTL()); err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	if err := s.sessions.RevokeRefreshToken(ctx, rawRefreshToken); err != nil {
		appErr := apperror.Internal(err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	s.auditLog(ctx, EventLogout, "", "", nil)
	log.InfoContext(ctx, "user logged out", slog.String("jti", jti))
	span.SetStatus(codes.Ok, "")
	return nil
}

// ForgotPassword initiates a password reset.
// Always returns nil regardless of whether the email is registered —
// this prevents account enumeration (Rule 12).
func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	ctx, span := authTracer.Start(ctx, "service/ForgotPassword")
	defer span.End()

	log := logger.FromContext(ctx)

	user, err := s.userSvc.GetByEmail(ctx, email)
	if err != nil {
		// User not found — emit audit event but still return nil to the caller.
		s.auditLog(ctx, EventPasswordResetRequest, email, "", map[string]any{"found": false})
		span.SetStatus(codes.Ok, "not found — enumeration prevented")
		return nil
	}

	rawToken, err := s.jwt.IssueRefreshToken() // reuse the 32-byte cryptographic random generator
	if err != nil {
		appErr := apperror.Internal(fmt.Errorf("generate reset token: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	if err := s.sessions.StorePasswordResetToken(ctx, rawToken, user.ID.String()); err != nil {
		appErr := apperror.Internal(fmt.Errorf("store reset token: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appBaseURL, rawToken)

	if s.mailer != nil {
		if mailErr := s.mailer.SendPasswordReset(ctx, email, resetURL); mailErr != nil {
			// Non-fatal — the token is stored; the user can retry the request.
			log.WarnContext(ctx, "failed to send reset email", slog.Any("error", mailErr))
		}
	}

	s.auditLog(ctx, EventPasswordResetRequest, email, user.ID.String(), nil)
	span.SetStatus(codes.Ok, "")
	return nil
}

// ResetPassword validates the reset token and sets the new password.
// The token is consumed (deleted) before the password update — it cannot be reused
// even if the password update subsequently fails.
func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	ctx, span := authTracer.Start(ctx, "service/ResetPassword")
	defer span.End()

	if len(newPassword) < 8 {
		appErr := apperror.BadRequest("password must be at least 8 characters", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	// Consume the token atomically — deleted immediately, cannot be reused.
	userIDStr, err := s.sessions.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		appErr := apperror.BadRequest("invalid or expired reset token", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	id, err := uuid.Parse(userIDStr)
	if err != nil {
		appErr := apperror.Internal(fmt.Errorf("invalid user ID in reset token: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		appErr := apperror.Internal(fmt.Errorf("hash password: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	if err := s.userSvc.UpdatePassword(ctx, id, string(hash)); err != nil {
		appErr := apperror.Internal(fmt.Errorf("update password: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return appErr
	}

	s.auditLog(ctx, EventPasswordResetDone, "", userIDStr, nil)
	span.SetStatus(codes.Ok, "")
	return nil
}

// --- Audit log helpers ---

// auditLog emits a structured audit event. It never blocks or returns an error —
// logging failure must not affect the auth operation (Rule 10).
func (s *authService) auditLog(ctx context.Context, eventType, email, userID string, extra map[string]any) {
	log := logger.FromContext(ctx)

	attrs := []any{
		slog.String("event_type", eventType),
		slog.String("ip", reqctx.IPFromContext(ctx)),
		slog.String("user_agent", reqctx.UserAgentFromContext(ctx)),
	}
	if email != "" {
		attrs = append(attrs, slog.String("email_masked", maskEmail(email)))
	}
	if userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}
	for k, v := range extra {
		attrs = append(attrs, slog.Any(k, v))
	}
	log.InfoContext(ctx, "audit", attrs...)
}

// maskEmail masks an email address for use in audit logs.
// Masking prevents raw PII from appearing in log aggregation systems.
//
// Examples:
//
//	user@example.com  →  u***@e***.com
//	admin@corp.io     →  a***@c***.io
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || len(parts[0]) == 0 {
		return "***"
	}
	local, domain := parts[0], parts[1]

	maskedLocal := string(local[0]) + "***"

	domainParts := strings.SplitN(domain, ".", 2)
	if len(domainParts[0]) == 0 {
		return maskedLocal + "@***"
	}
	maskedDomain := string(domainParts[0][0]) + "***"
	if len(domainParts) > 1 {
		maskedDomain += "." + domainParts[1]
	}
	return maskedLocal + "@" + maskedDomain
}

// userAgentFromCtx is a convenience shim used by auditLog.
func userAgentFromCtx(ctx context.Context) string {
	return reqctx.UserAgentFromContext(ctx)
}

// ipFromCtx is a convenience shim used by auditLog.
func ipFromCtx(ctx context.Context) string {
	return reqctx.IPFromContext(ctx)
}

// Compile-time check that unused shims satisfy the linter.
var _ = userAgentFromCtx
var _ = ipFromCtx
