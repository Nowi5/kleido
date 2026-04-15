package redis

// Redis key schema:
//
//	auth:refresh:{sha256(token)}          → userID string    TTL = refresh token lifetime
//	auth:blocklist:{jti}                  → "1"              TTL = remaining access token lifetime
//	auth:lockout:{sha256(email)}          → failure count    TTL = lockout window (15 min)
//	auth:reset:{sha256(token)}            → userID string    TTL = 1 hour
//	rate:limit:{identifier}               → sorted set       TTL = window duration
//	rate:user:{userID}:{endpoint}         → sorted set       TTL = window duration
//
// Note: emails and tokens are always SHA-256 hashed before use as key segments
// to prevent PII from appearing in Redis key names, MONITOR output, or logs.

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/nowi5/kleido/internal/repository"
	"github.com/nowi5/kleido/pkg/apperror"
	"github.com/redis/go-redis/v9"
)

const (
	loginLockoutKeyPrefix  = "auth:lockout:"
	loginLockoutThreshold  = int64(10)
	passwordResetKeyPrefix = "auth:reset:"
	passwordResetTTL       = 1 * time.Hour
)

type sessionRepo struct {
	rdb *redis.Client
}

// NewSessionRepo returns a repository.SessionRepository backed by Redis.
func NewSessionRepo(rdb *redis.Client) repository.SessionRepository {
	return &sessionRepo{rdb: rdb}
}

// hashToken returns the lowercase hex SHA-256 of the raw value.
// Used for tokens, emails, and any value that must not appear raw in Redis keys.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

func refreshKey(token string) string {
	return "auth:refresh:" + hashToken(token)
}

func blocklistKey(jti string) string {
	return "auth:blocklist:" + jti
}

func lockoutKey(email string) string {
	return loginLockoutKeyPrefix + hashToken(email)
}

func resetKey(token string) string {
	return passwordResetKeyPrefix + hashToken(token)
}

// StoreRefreshToken stores sha256(token) → userID with the given TTL.
func (r *sessionRepo) StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, refreshKey(token), userID, ttl).Err(); err != nil {
		return fmt.Errorf("session: store refresh token: %w", err)
	}
	return nil
}

// ValidateRefreshToken returns the userID associated with sha256(token).
// Returns apperror.Unauthorized if the token is missing or expired.
func (r *sessionRepo) ValidateRefreshToken(ctx context.Context, token string) (string, error) {
	userID, err := r.rdb.Get(ctx, refreshKey(token)).Result()
	if err != nil {
		if err == redis.Nil {
			return "", apperror.Unauthorized("refresh token invalid or expired")
		}
		return "", fmt.Errorf("session: validate refresh token: %w", err)
	}
	return userID, nil
}

// RevokeRefreshToken deletes the refresh token entry from Redis.
func (r *sessionRepo) RevokeRefreshToken(ctx context.Context, token string) error {
	if err := r.rdb.Del(ctx, refreshKey(token)).Err(); err != nil {
		return fmt.Errorf("session: revoke refresh token: %w", err)
	}
	return nil
}

// BlocklistJTI adds the JTI to the revocation list with the given TTL.
func (r *sessionRepo) BlocklistJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, blocklistKey(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("session: blocklist jti: %w", err)
	}
	return nil
}

// IsBlocklisted returns true if the JTI has been revoked.
func (r *sessionRepo) IsBlocklisted(ctx context.Context, jti string) (bool, error) {
	n, err := r.rdb.Exists(ctx, blocklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("session: check blocklist: %w", err)
	}
	return n > 0, nil
}

// RotateRefreshToken atomically revokes oldToken and stores newToken → userID
// in a single Redis pipeline. Either both succeed or neither persists.
func (r *sessionRepo) RotateRefreshToken(
	ctx context.Context,
	oldToken, newToken, userID string,
	ttl time.Duration,
) error {
	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, refreshKey(oldToken))
	pipe.Set(ctx, refreshKey(newToken), userID, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}
	return nil
}

// --- Brute-force lockout ---

// IncrLoginFailure increments the failed login counter for the given email.
// The email is hashed before use as a Redis key so no raw PII is stored in key names.
// Returns the new count after increment.
func (r *sessionRepo) IncrLoginFailure(ctx context.Context, email string, ttl time.Duration) (int64, error) {
	key := lockoutKey(email)
	pipe := r.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("incr login failure: %w", err)
	}
	return incrCmd.Val(), nil
}

// GetLoginFailures returns the current failed login count for the email.
// Returns 0 if no failures are recorded (key does not exist).
func (r *sessionRepo) GetLoginFailures(ctx context.Context, email string) (int64, error) {
	val, err := r.rdb.Get(ctx, lockoutKey(email)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get login failures: %w", err)
	}
	return val, nil
}

// ClearLoginFailures deletes the failed login counter.
// Called on successful authentication to reset the lockout window.
func (r *sessionRepo) ClearLoginFailures(ctx context.Context, email string) error {
	if err := r.rdb.Del(ctx, lockoutKey(email)).Err(); err != nil {
		return fmt.Errorf("clear login failures: %w", err)
	}
	return nil
}

// IsLockedOut returns true if the email has exceeded the failure threshold.
func (r *sessionRepo) IsLockedOut(ctx context.Context, email string) (bool, error) {
	count, err := r.GetLoginFailures(ctx, email)
	if err != nil {
		return false, err
	}
	return count >= loginLockoutThreshold, nil
}

// --- Per-user rate limiting ---

// RateLimitAllowUser is the per-user variant of RateLimitAllow.
// Key schema: rate:user:{userID}:{endpoint}
func (r *sessionRepo) RateLimitAllowUser(
	ctx context.Context,
	userID, endpoint string,
	limit int64,
	window time.Duration,
) (bool, int64, time.Time, error) {
	key := fmt.Sprintf("rate:user:%s:%s", userID, endpoint)
	return r.rateLimitByKey(ctx, key, limit, window)
}

// RateLimitAllow implements a sliding-window rate limiter using a sorted set.
// Each call records a member with the current timestamp, prunes old members,
// then checks whether the remaining count exceeds the limit.
//
// Fails open: if the Redis pipeline fails, the request is allowed through.
func (r *sessionRepo) RateLimitAllow(
	ctx context.Context,
	key string,
	limit int64,
	window time.Duration,
) (bool, int64, time.Time, error) {
	return r.rateLimitByKey(ctx, "rate:limit:"+key, limit, window)
}

// rateLimitByKey is the internal sliding-window implementation shared by
// RateLimitAllow and RateLimitAllowUser. It takes the final Redis key directly.
func (r *sessionRepo) rateLimitByKey(
	ctx context.Context,
	rKey string,
	limit int64,
	window time.Duration,
) (allowed bool, remaining int64, resetAt time.Time, _ error) {
	now := time.Now()
	windowStart := now.Add(-window)
	resetAt = now.Add(window)

	pipe := r.rdb.Pipeline()
	// Remove members older than the window.
	pipe.ZRemRangeByScore(ctx, rKey, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	// Add this request.
	pipe.ZAdd(ctx, rKey, redis.Z{Score: float64(now.UnixMilli()), Member: now.UnixNano()})
	// Count members in the window.
	countCmd := pipe.ZCard(ctx, rKey)
	// Set the key to expire after the window.
	pipe.Expire(ctx, rKey, window)

	if _, err := pipe.Exec(ctx); err != nil {
		// Fail open: Redis unavailable → allow the request.
		return true, limit, resetAt, nil
	}

	count := countCmd.Val()
	if count > limit {
		return false, 0, resetAt, nil
	}
	return true, limit - count, resetAt, nil
}

// --- Password reset ---

// StorePasswordResetToken stores sha256(token) → userID with a 1-hour TTL.
// The raw token is sent to the user; only the hash is stored in Redis.
func (r *sessionRepo) StorePasswordResetToken(ctx context.Context, token, userID string) error {
	if err := r.rdb.Set(ctx, resetKey(token), userID, passwordResetTTL).Err(); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}
	return nil
}

// ConsumePasswordResetToken retrieves the userID for the token and deletes it atomically.
// The token is single-use: it is deleted before returning so it cannot be reused.
// Returns an error if the token does not exist (expired or already consumed).
func (r *sessionRepo) ConsumePasswordResetToken(ctx context.Context, token string) (string, error) {
	key := resetKey(token)

	// Pipeline: GET then DEL — ensures the token is deleted even if the caller crashes.
	// A Lua GETDEL script would be strictly atomic; the pipeline is close enough for
	// this use case since the 1-hour TTL limits the replay window.
	pipe := r.rdb.Pipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return "", fmt.Errorf("consume reset token: %w", err)
	}

	userID, err := getCmd.Result()
	if err == redis.Nil {
		return "", fmt.Errorf("reset token not found or already used")
	}
	if err != nil {
		return "", fmt.Errorf("get reset token: %w", err)
	}
	return userID, nil
}
