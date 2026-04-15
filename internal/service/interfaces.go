// Package service contains the business logic layer of kleido.
// It depends only on repository interfaces, never on concrete implementations.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"kleido/internal/model"
)

// UserService is the contract for user business logic.
type UserService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, email, password, role string) (*model.User, error)
	List(ctx context.Context, limit, offset int) ([]*model.UserResponse, int64, error)
	// Update applies a partial update. callerRole controls whether the role field
	// may be changed (only "admin" callers may alter roles).
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateUserRequest, callerRole string) (*model.User, error)
	// Delete soft-deletes the user and invalidates the cache entry.
	Delete(ctx context.Context, id uuid.UUID) error
	// UpdatePassword sets a new (already bcrypt-hashed) password for the user.
	// Invalidates the user cache entry.
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

// AuthService is the contract for authentication business logic.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*model.User, error)
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, jti, rawRefreshToken string) error
	// ForgotPassword initiates a password reset.
	// Always returns nil — even if the email is not registered — to prevent enumeration.
	ForgotPassword(ctx context.Context, email string) error
	// ResetPassword validates the reset token and sets the new password.
	// Returns apperror.BadRequest if the token is invalid/expired or the password is too short.
	ResetPassword(ctx context.Context, token, newPassword string) error
}

// TokenPair holds the result of a successful login or refresh.
type TokenPair struct {
	AccessToken string
	JTI         string
	ExpiresAt   time.Time
	// RawRefreshToken is the NEW refresh token after rotation.
	// The old token passed to Refresh is revoked atomically.
	// sha256(RawRefreshToken) is stored in Redis — never the raw value.
	RawRefreshToken string
}
