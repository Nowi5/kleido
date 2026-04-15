// Package repository defines the storage interfaces implemented by the
// postgres and redis sub-packages.
package repository

import (
	"context"
	"time"

	"kleido/internal/model"

	"github.com/google/uuid"
)

// UserRepository defines the data-access contract for users.
// The concrete implementation lives in internal/repository/postgres.
// Services depend on this interface, never on the concrete struct.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	// UpdatePassword sets a new password_hash for the given user ID.
	// It does not affect any other field.
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*model.User, int64, error)
}

// TenantRepository defines the data-access contract for tenants.
type TenantRepository interface {
	Create(ctx context.Context, tenant *model.Tenant) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*model.Tenant, error)
	List(ctx context.Context) ([]*model.Tenant, error)
}

// SessionRepository manages JWT token lifecycle in Redis.
type SessionRepository interface {
	// StoreRefreshToken stores sha256(token) → userID with TTL.
	StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error
	// ValidateRefreshToken returns the userID for a valid token, or error if expired/missing.
	ValidateRefreshToken(ctx context.Context, token string) (string, error)
	// RevokeRefreshToken deletes the refresh token entry.
	RevokeRefreshToken(ctx context.Context, token string) error
	// BlocklistJTI adds an access token's JTI to the blocklist until expiry.
	BlocklistJTI(ctx context.Context, jti string, ttl time.Duration) error
	// IsBlocklisted returns true if the JTI has been revoked.
	IsBlocklisted(ctx context.Context, jti string) (bool, error)
	// RateLimitAllow checks and records a rate limit hit using a sliding window.
	// Returns (allowed, remaining, resetAt, error). Fails open on Redis error.
	RateLimitAllow(ctx context.Context, key string, limit int64, window time.Duration) (bool, int64, time.Time, error)
	// RotateRefreshToken atomically revokes oldToken and stores newToken → userID.
	// Both operations execute in a single Redis pipeline so a partial failure
	// does not leave a stale token alive.
	RotateRefreshToken(ctx context.Context, oldToken, newToken, userID string, ttl time.Duration) error

	// --- Brute-force lockout ---

	// IncrLoginFailure increments the failed login counter for the given email.
	// The email is hashed before use as a key — no raw PII in Redis key names.
	// Returns the new count. The counter expires automatically after ttl.
	IncrLoginFailure(ctx context.Context, email string, ttl time.Duration) (int64, error)
	// GetLoginFailures returns the current failed login count for the email.
	// Returns 0 if no failures are recorded.
	GetLoginFailures(ctx context.Context, email string) (int64, error)
	// ClearLoginFailures deletes the failed login counter (called on successful login).
	ClearLoginFailures(ctx context.Context, email string) error
	// IsLockedOut returns true if the email has exceeded the failure threshold (10 attempts).
	IsLockedOut(ctx context.Context, email string) (bool, error)

	// --- Per-user rate limiting ---

	// RateLimitAllowUser is the per-user variant of RateLimitAllow.
	// Key format: rate:user:{userID}:{endpoint}
	// Unlike the IP limiter, this key uses the authenticated user's ID.
	RateLimitAllowUser(ctx context.Context, userID, endpoint string, limit int64, window time.Duration) (bool, int64, time.Time, error)

	// --- Password reset ---

	// StorePasswordResetToken stores sha256(token) → userID with a 1-hour TTL.
	StorePasswordResetToken(ctx context.Context, token, userID string) error
	// ConsumePasswordResetToken retrieves the userID for the token and deletes it atomically.
	// Returns an error if the token does not exist (expired or already used).
	ConsumePasswordResetToken(ctx context.Context, token string) (userID string, err error)
}

// CacheRepository provides generic JSON caching.
type CacheRepository interface {
	SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error
	// GetJSON returns redis.Nil if the key does not exist.
	GetJSON(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
}
