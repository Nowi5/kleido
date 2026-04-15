// Package handler implements the HTTP handlers for the kleido API.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"kleido/internal/auth"
	"kleido/internal/logger"
	"kleido/internal/middleware"
	"kleido/internal/service"
	"kleido/pkg/apperror"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var authTracer = otel.Tracer("kleido/handler")

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	svc                 service.AuthService
	jwtSvc              *auth.JWTService
	isProd              bool
	registrationEnabled bool
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc service.AuthService, jwtSvc *auth.JWTService, isProd bool, registrationEnabled bool) *AuthHandler {
	return &AuthHandler{svc: svc, jwtSvc: jwtSvc, isProd: isProd, registrationEnabled: registrationEnabled}
}

// RegisterRequest is the payload for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email"    example:"newuser@example.com"`
	Password string `json:"password" example:"Password1!"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"    example:"user@example.com"`
	Password string `json:"password" example:"Password1!"`
}

// TokenResponse is the JSON body returned by Login and Refresh.
type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// encodeJSON writes v as JSON to w, logging any encoding error.
// It must only be called after the response header has been written.
func encodeJSON(w http.ResponseWriter, r *http.Request, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.FromContext(r.Context()).WarnContext(r.Context(), "encode response", slog.Any("error", err))
	}
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Creates a new user account with the "user" role.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		RegisterRequest	true	"Registration payload"
//	@Success		201		{object}	model.UserResponse
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		409		{object}	apperror.ErrorResponse	"Email already registered"
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// No user-identifying attributes on auth spans — credentials must not leak.
	ctx, span := authTracer.Start(r.Context(), "handler/Register")
	defer span.End()

	if !h.registrationEnabled {
		appErr := apperror.Forbidden("user registration is disabled")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	log := logger.FromContext(ctx)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := apperror.BadRequest("invalid request body", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}
	if req.Email == "" || req.Password == "" {
		appErr := apperror.BadRequest("email and password are required", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	user, err := h.svc.Register(ctx, req.Email, req.Password)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	log.InfoContext(ctx, "user registered", slog.String("user_id", user.ID.String()))
	span.SetStatus(codes.Ok, "")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encodeJSON(w, r, user.ToResponse())
}

// Login godoc
//
//	@Summary		Authenticate a user
//	@Description	Validates credentials and returns an access token. The refresh token is set as an httpOnly cookie.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"Login payload"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{object}	apperror.ErrorResponse	"Missing or malformed request body"
//	@Failure		401		{object}	apperror.ErrorResponse	"Invalid credentials"
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// No user-identifying attributes on auth spans — credentials must not leak.
	ctx, span := authTracer.Start(r.Context(), "handler/Login")
	defer span.End()

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := apperror.BadRequest("invalid request body", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}
	if req.Email == "" || req.Password == "" {
		appErr := apperror.BadRequest("email and password are required", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	pair, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	// Set the refresh token in an httpOnly cookie only — never in the body.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RawRefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(h.jwtSvc.RefreshTTL()),
		MaxAge:   int(h.jwtSvc.RefreshTTL().Seconds()),
	})

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, TokenResponse{
		AccessToken: pair.AccessToken,
		ExpiresAt:   pair.ExpiresAt,
	})
}

// Refresh godoc
//
//	@Summary		Refresh access token
//	@Description	Issues a new access token using the refresh_token cookie. The old refresh token is revoked and replaced with a new one (rotation).
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	TokenResponse			"New access token; new refresh_token cookie set"
//	@Failure		401	{object}	apperror.ErrorResponse	"Missing or invalid refresh token cookie"
//	@Router			/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// No user-identifying attributes on auth spans — tokens must not leak.
	ctx, span := authTracer.Start(r.Context(), "handler/Refresh")
	defer span.End()

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		appErr := apperror.Unauthorized("refresh token cookie missing")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}
	if cookie.Value == "" {
		appErr := apperror.Unauthorized("refresh token cookie empty")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	pair, err := h.svc.Refresh(ctx, cookie.Value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	// Issue a new refresh token cookie — the old token has been rotated out.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RawRefreshToken,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(h.jwtSvc.RefreshTTL()),
		MaxAge:   int(h.jwtSvc.RefreshTTL().Seconds()),
	})

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, TokenResponse{
		AccessToken: pair.AccessToken,
		ExpiresAt:   pair.ExpiresAt,
	})
}

// ForgotPasswordRequest is the payload for POST /auth/forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the payload for POST /auth/reset-password.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ForgotPassword godoc
//
//	@Summary		Request password reset
//	@Description	Sends a password reset email. Always returns 200 regardless of whether
//	@Description	the email is registered, to prevent account enumeration.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ForgotPasswordRequest	true	"Email address"
//	@Success		200		{object}	map[string]string		"message: if your email is registered, a reset link has been sent"
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Router			/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx, span := authTracer.Start(r.Context(), "handler/ForgotPassword")
	defer span.End()

	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := apperror.BadRequest("invalid request body", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}
	if req.Email == "" {
		appErr := apperror.BadRequest("email is required", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	_ = h.svc.ForgotPassword(ctx, req.Email) //nolint:errcheck

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, map[string]string{
		"message": "if your email is registered, a reset link has been sent",
	})
}

// ResetPassword godoc
//
//	@Summary		Reset password using token
//	@Description	Sets a new password using a valid reset token from the email link.
//	@Description	The token is single-use and expires after 1 hour.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		ResetPasswordRequest	true	"Reset token and new password"
//	@Success		200		{object}	map[string]string		"message: password updated"
//	@Failure		400		{object}	apperror.ErrorResponse	"Invalid/expired token or weak password"
//	@Router			/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx, span := authTracer.Start(r.Context(), "handler/ResetPassword")
	defer span.End()

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := apperror.BadRequest("invalid request body", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		appErr := apperror.BadRequest("token and new_password are required", nil)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	if err := h.svc.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, map[string]string{"message": "password updated"})
}

// Logout godoc
//
//	@Summary		Revoke the current session
//	@Description	Blocklists the access token's JTI and revokes the refresh token. Clears the refresh_token cookie.
//	@Tags			auth
//	@Produce		json
//	@Success		204	"No content"
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// No user-identifying attributes on auth spans — tokens must not leak.
	ctx, span := authTracer.Start(r.Context(), "handler/Logout")
	defer span.End()

	jti, ok := ctx.Value(middleware.CtxKeyJTI).(string)
	if !ok {
		jti = ""
	}

	var rawRefresh string
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		rawRefresh = cookie.Value
	}

	if err := h.svc.Logout(ctx, jti, rawRefresh); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	// Clear the refresh cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.isProd,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	span.SetStatus(codes.Ok, "")
	w.WriteHeader(http.StatusNoContent)
}
