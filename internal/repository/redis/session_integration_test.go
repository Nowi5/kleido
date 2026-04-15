//go:build integration

package redis_test

import (
	"context"
	"testing"
	"time"

	redisrepo "kleido/internal/repository/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testRedis starts a redis:7-alpine container and returns a connected client.
// The container is terminated via t.Cleanup.
func testRedis(t *testing.T) *goredis.Client {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr: host + ":" + port.Port(),
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	return rdb
}

func TestStoreAndValidateRefreshToken(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	token := "raw-refresh-token-abc123"
	userID := "user-uuid-001"

	if err := repo.StoreRefreshToken(ctx, token, userID, 5*time.Minute); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}

	got, err := repo.ValidateRefreshToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if got != userID {
		t.Errorf("userID: want %q, got %q", userID, got)
	}
}

func TestValidateRefreshToken_Unknown(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	_, err := repo.ValidateRefreshToken(ctx, "does-not-exist")
	if err == nil {
		t.Error("expected error for unknown token, got nil")
	}
}

func TestRevokeRefreshToken(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	token := "revoke-me-token"
	if err := repo.StoreRefreshToken(ctx, token, "user-001", 5*time.Minute); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}
	if err := repo.RevokeRefreshToken(ctx, token); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}

	_, err := repo.ValidateRefreshToken(ctx, token)
	if err == nil {
		t.Error("expected error after revocation, got nil")
	}
}

func TestBlocklistJTI(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	jti := "test-jti-abc"
	if err := repo.BlocklistJTI(ctx, jti, 5*time.Minute); err != nil {
		t.Fatalf("BlocklistJTI: %v", err)
	}

	blocked, err := repo.IsBlocklisted(ctx, jti)
	if err != nil {
		t.Fatalf("IsBlocklisted: %v", err)
	}
	if !blocked {
		t.Error("expected JTI to be blocklisted")
	}
}

func TestIsBlocklisted_Unknown(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	blocked, err := repo.IsBlocklisted(ctx, "unknown-jti")
	if err != nil {
		t.Fatalf("IsBlocklisted: %v", err)
	}
	if blocked {
		t.Error("unknown JTI must not be blocklisted")
	}
}

func TestRateLimitAllow_SlidingWindow(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	key := "test-ip-rate-limit"
	limit := int64(3)
	window := 10 * time.Second

	for i := 0; i < 3; i++ {
		allowed, _, _, err := repo.RateLimitAllow(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("call %d: RateLimitAllow: %v", i+1, err)
		}
		if !allowed {
			t.Errorf("call %d: expected allowed=true, got false", i+1)
		}
	}

	// 4th and 5th calls should be denied.
	for i := 3; i < 5; i++ {
		allowed, remaining, _, err := repo.RateLimitAllow(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("call %d: RateLimitAllow: %v", i+1, err)
		}
		if allowed {
			t.Errorf("call %d: expected allowed=false, got true", i+1)
		}
		if remaining != 0 {
			t.Errorf("call %d: remaining should be 0, got %d", i+1, remaining)
		}
	}
}

func TestRotateRefreshToken(t *testing.T) {

	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	oldToken := "rotate-old-token"
	newToken := "rotate-new-token"
	userID := "rotate-user-uuid"

	// Store the old token first.
	if err := repo.StoreRefreshToken(ctx, oldToken, userID, 5*time.Minute); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}

	// Rotate: old → new.
	if err := repo.RotateRefreshToken(ctx, oldToken, newToken, userID, 5*time.Minute); err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}

	// Old token must no longer be valid.
	_, err := repo.ValidateRefreshToken(ctx, oldToken)
	if err == nil {
		t.Error("old token must be invalid after rotation")
	}

	// New token must be valid and return the correct userID.
	gotID, err := repo.ValidateRefreshToken(ctx, newToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken(new): %v", err)
	}
	if gotID != userID {
		t.Errorf("userID: want %q, got %q", userID, gotID)
	}
}

func TestRateLimitAllow_FailOpen(t *testing.T) {

	// Create a client pointing at a non-existent Redis to simulate unavailability.
	rdb := goredis.NewClient(&goredis.Options{
		Addr:        "localhost:19999",
		DialTimeout: 100 * time.Millisecond,
	})

	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	allowed, _, _, err := repo.RateLimitAllow(ctx, "fail-open-key", 10, time.Minute)
	if err != nil {
		t.Fatalf("expected no error on fail-open, got: %v", err)
	}
	if !allowed {
		t.Error("fail-open: expected allowed=true when Redis is unavailable")
	}
}

func TestIncrLoginFailure(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	email := "bruteforce@example.com"
	ttl := 5 * time.Minute

	for i := 1; i <= 3; i++ {
		count, err := repo.IncrLoginFailure(ctx, email, ttl)
		if err != nil {
			t.Fatalf("IncrLoginFailure %d: %v", i, err)
		}
		if count != int64(i) {
			t.Errorf("count: want %d, got %d", i, count)
		}
	}
}

func TestGetLoginFailures(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	email := "getfailures@example.com"
	ttl := 5 * time.Minute

	count, err := repo.GetLoginFailures(ctx, email)
	if err != nil {
		t.Fatalf("GetLoginFailures: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for new email, got %d", count)
	}

	for i := 0; i < 5; i++ {
		if _, err := repo.IncrLoginFailure(ctx, email, ttl); err != nil {
			t.Fatalf("IncrLoginFailure: %v", err)
		}
	}

	count, err = repo.GetLoginFailures(ctx, email)
	if err != nil {
		t.Fatalf("GetLoginFailures after increments: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestClearLoginFailures(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	email := "clear@example.com"
	ttl := 5 * time.Minute

	for i := 0; i < 7; i++ {
		if _, err := repo.IncrLoginFailure(ctx, email, ttl); err != nil {
			t.Fatalf("IncrLoginFailure: %v", err)
		}
	}

	if err := repo.ClearLoginFailures(ctx, email); err != nil {
		t.Fatalf("ClearLoginFailures: %v", err)
	}

	count, err := repo.GetLoginFailures(ctx, email)
	if err != nil {
		t.Fatalf("GetLoginFailures after clear: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

func TestIsLockedOut_Threshold(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	email := "lockout@example.com"
	ttl := 5 * time.Minute

	locked, err := repo.IsLockedOut(ctx, email)
	if err != nil {
		t.Fatalf("IsLockedOut (initial): %v", err)
	}
	if locked {
		t.Error("expected false for new email")
	}

	for i := 0; i < 9; i++ {
		if _, err := repo.IncrLoginFailure(ctx, email, ttl); err != nil {
			t.Fatalf("IncrLoginFailure: %v", err)
		}
	}

	locked, err = repo.IsLockedOut(ctx, email)
	if err != nil {
		t.Fatalf("IsLockedOut (at 9): %v", err)
	}
	if locked {
		t.Error("expected false at 9 failures (threshold is 10)")
	}

	if _, err := repo.IncrLoginFailure(ctx, email, ttl); err != nil {
		t.Fatalf("IncrLoginFailure 10th: %v", err)
	}

	locked, err = repo.IsLockedOut(ctx, email)
	if err != nil {
		t.Fatalf("IsLockedOut (at 10): %v", err)
	}
	if !locked {
		t.Error("expected true at 10 failures")
	}
}

func TestStoreAndConsumePasswordResetToken(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	token := "password-reset-token-abc123"
	userID := "user-reset-uuid"

	if err := repo.StorePasswordResetToken(ctx, token, userID); err != nil {
		t.Fatalf("StorePasswordResetToken: %v", err)
	}

	gotID, err := repo.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		t.Fatalf("ConsumePasswordResetToken: %v", err)
	}
	if gotID != userID {
		t.Errorf("userID: want %q, got %q", userID, gotID)
	}
}

func TestConsumePasswordResetToken_AlreadyUsed(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	token := "single-use-token-xyz"
	userID := "user-single-use"

	if err := repo.StorePasswordResetToken(ctx, token, userID); err != nil {
		t.Fatalf("StorePasswordResetToken: %v", err)
	}

	_, err := repo.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}

	_, err = repo.ConsumePasswordResetToken(ctx, token)
	if err == nil {
		t.Error("expected error on second consume (token already used), got nil")
	}
}

func TestConsumePasswordResetToken_Unknown(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	_, err := repo.ConsumePasswordResetToken(ctx, "non-existent-token")
	if err == nil {
		t.Error("expected error for unknown token, got nil")
	}
}

func TestRateLimitAllowUser(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	userID := "user-rate-limit-001"
	endpoint := "/api/v1/protected"
	limit := int64(2)
	window := 10 * time.Second

	for i := 0; i < 2; i++ {
		allowed, _, _, err := repo.RateLimitAllowUser(ctx, userID, endpoint, limit, window)
		if err != nil {
			t.Fatalf("call %d: RateLimitAllowUser: %v", i+1, err)
		}
		if !allowed {
			t.Errorf("call %d: expected allowed=true, got false", i+1)
		}
	}

	allowed, remaining, _, err := repo.RateLimitAllowUser(ctx, userID, endpoint, limit, window)
	if err != nil {
		t.Fatalf("3rd call: %v", err)
	}
	if allowed {
		t.Error("3rd call: expected allowed=false")
	}
	if remaining != 0 {
		t.Errorf("remaining: want 0, got %d", remaining)
	}
}

func TestRateLimitAllowUser_DifferentEndpoints(t *testing.T) {
	rdb := testRedis(t)
	repo := redisrepo.NewSessionRepo(rdb)
	ctx := context.Background()

	userID := "user-multi-endpoint"
	limit := int64(1)
	window := 10 * time.Second

	_, _, _, err := repo.RateLimitAllowUser(ctx, userID, "/api/endpoint-a", limit, window)
	if err != nil {
		t.Fatalf("endpoint A: %v", err)
	}

	allowed, _, _, err := repo.RateLimitAllowUser(ctx, userID, "/api/endpoint-b", limit, window)
	if err != nil {
		t.Fatalf("endpoint B: %v", err)
	}
	if !allowed {
		t.Error("different endpoint: expected allowed=true (separate rate limit)")
	}
}
