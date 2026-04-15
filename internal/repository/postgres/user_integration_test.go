//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"kleido/internal/config"
	"kleido/internal/model"
	"kleido/internal/repository/postgres"
	"kleido/pkg/apperror"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testDB spins up a postgres:16-alpine container, runs migrations, and returns
// the connection string. The container is terminated in t.Cleanup.
func testDB(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForExposedPort().WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
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

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
PollDB:
	for readyCtx.Err() == nil {
		exitCode, _, err := container.Exec(readyCtx, []string{"pg_isready", "-U", "testuser", "-d", "testdb"})
		if err == nil && exitCode == 0 {
			break PollDB
		}
		select {
		case <-readyCtx.Done():
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
	if readyCtx.Err() != nil {
		t.Fatalf("pg_isready: database not ready within 30s")
	}

	migCtx, migCancel := context.WithTimeout(ctx, 30*time.Second)
	defer migCancel()
	var migErr error
RetryMigrate:
	for i := 0; i < 5; i++ {
		migErr = postgres.RunMigrations(dsn, "../../../migrations")
		if migErr == nil {
			break RetryMigrate
		}
		select {
		case <-migCtx.Done():
			break RetryMigrate
		default:
			time.Sleep(time.Duration(1+i) * time.Second)
		}
	}
	if migErr != nil {
		t.Fatalf("run migrations: %v", migErr)
	}

	return dsn
}

func newTestPool(t *testing.T, dsn string) *postgres_internal_pool {
	t.Helper()
	// We import the pool indirectly via config to avoid exposing internals.
	// Use a helper that wraps NewPool.
	return &postgres_internal_pool{dsn: dsn}
}

// We need access to pgxpool.Pool for the repository constructor, so we import
// the postgres package and use NewPool directly.
func newPool(t *testing.T, dsn string) interface{ Close() } {
	t.Helper()
	// Imported via postgres.NewPool in the actual test functions.
	return nil
}

func TestCreate_FindByEmail_RoundTrip(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	user := &model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("user-%s@test.com", uuid.New()),
		PasswordHash: "$2a$10$somehash",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}

	if got.ID != user.ID {
		t.Errorf("ID: want %v, got %v", user.ID, got.ID)
	}
	if got.Email != user.Email {
		t.Errorf("Email: want %q, got %q", user.Email, got.Email)
	}
	if got.Role != user.Role {
		t.Errorf("Role: want %q, got %q", user.Role, got.Role)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	_, err = repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected an error for non-existent user, got nil")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected apperror.NotFound, got: %v", err)
	}
}

func TestList_TotalCount(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Insert 3 users.
	for i := 0; i < 3; i++ {
		u := &model.User{
			ID:           uuid.New(),
			Email:        fmt.Sprintf("user-%s@test.com", uuid.New()),
			PasswordHash: "$2a$10$somehash",
			Role:         "user",
			IsActive:     true,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create user %d: %v", i, err)
		}
	}

	users, total, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if total < 3 {
		t.Errorf("total count: want >= 3, got %d", total)
	}
	if len(users) < 3 {
		t.Errorf("returned users: want >= 3, got %d", len(users))
	}
}

func TestDelete_SoftDelete(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	user := &model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("user-%s@test.com", uuid.New()),
		PasswordHash: "$2a$10$somehash",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// FindByID filters on is_active = true, so it must return NotFound.
	_, err = repo.FindByID(ctx, user.ID)
	if err == nil {
		t.Fatal("expected NotFound after soft delete, got nil")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected apperror.NotFound, got: %v", err)
	}

	// FindByEmail does NOT filter on is_active, so the row must still be there.
	got, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("FindByEmail after soft delete: %v", err)
	}
	if got.IsActive {
		t.Error("soft-deleted user must have is_active = false")
	}
}

// postgres_internal_pool is a placeholder type used only to make the file
// compile; the real pool is obtained from postgres.NewPool in each test.
type postgres_internal_pool struct{ dsn string }

func TestUpdate(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	user := &model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("update-test-%s@test.com", uuid.New()),
		PasswordHash: "$2a$10$oldhash",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	user.Role = "admin"
	user.Email = fmt.Sprintf("updated-%s@test.com", uuid.New())

	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}

	if got.Role != "admin" {
		t.Errorf("Role: want %q, got %q", "admin", got.Role)
	}
	if got.Email != user.Email {
		t.Errorf("Email: want %q, got %q", user.Email, got.Email)
	}
	if got.IsActive != true {
		t.Errorf("IsActive: want true, got %v", got.IsActive)
	}
}

func TestUpdatePassword(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	user := &model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("pw-update-%s@test.com", uuid.New()),
		PasswordHash: "$2a$10$oldhash",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newHash := "$2a$10$newpasswordhash"
	if err := repo.UpdatePassword(ctx, user.ID, newHash); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	got, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("FindByEmail after password update: %v", err)
	}

	if got.PasswordHash != newHash {
		t.Errorf("PasswordHash: want %q, got %q", newHash, got.PasswordHash)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	dsn := testDB(t)
	ctx := context.Background()

	cfg := config.DatabaseConfig{
		URL:                    dsn,
		MaxConns:               5,
		MinConns:               1,
		MaxConnLifetimeMinutes: 5,
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	user := &model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("ghost-%s@test.com", uuid.New()),
		PasswordHash: "$2a$10$somehash",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	err = repo.Update(ctx, user)
	if err != nil {
		t.Errorf("Update on non-existent user should not return error (no rows affected), got: %v", err)
	}
}
