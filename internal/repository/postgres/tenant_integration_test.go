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
)

func TestTenantCreate_FindByID_RoundTrip(t *testing.T) {
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

	repo := postgres.NewTenantRepository(pool)

	tenant := &model.Tenant{
		ID:       uuid.New(),
		Name:     "Acme Corporation",
		Slug:     fmt.Sprintf("acme-%s", uuid.New().String()[:8]),
		Settings: map[string]interface{}{"theme": "dark", "locale": "en-US"},
		IsActive: true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if got.ID != tenant.ID {
		t.Errorf("ID: want %v, got %v", tenant.ID, got.ID)
	}
	if got.Name != tenant.Name {
		t.Errorf("Name: want %q, got %q", tenant.Name, got.Name)
	}
	if got.Slug != tenant.Slug {
		t.Errorf("Slug: want %q, got %q", tenant.Slug, got.Slug)
	}
}

func TestTenantFindBySlug(t *testing.T) {
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

	repo := postgres.NewTenantRepository(pool)

	tenant := &model.Tenant{
		ID:       uuid.New(),
		Name:     "Beta Industries",
		Slug:     fmt.Sprintf("beta-%s", uuid.New().String()[:8]),
		Settings: map[string]interface{}{},
		IsActive: true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindBySlug(ctx, tenant.Slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}

	if got.ID != tenant.ID {
		t.Errorf("ID: want %v, got %v", tenant.ID, got.ID)
	}
}

func TestTenantFindByID_NotFound(t *testing.T) {
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

	repo := postgres.NewTenantRepository(pool)

	_, err = repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected an error for non-existent tenant, got nil")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected apperror.NotFound, got: %v", err)
	}
}

func TestTenantFindBySlug_NotFound(t *testing.T) {
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

	repo := postgres.NewTenantRepository(pool)

	_, err = repo.FindBySlug(ctx, "non-existent-slug")
	if err == nil {
		t.Fatal("expected an error for non-existent slug, got nil")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected apperror.NotFound, got: %v", err)
	}
}

func TestTenantList(t *testing.T) {
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

	repo := postgres.NewTenantRepository(pool)

	for i := 0; i < 3; i++ {
		tenant := &model.Tenant{
			ID:        uuid.New(),
			Name:      fmt.Sprintf("Tenant %d", i),
			Slug:      fmt.Sprintf("tenant-%s-%d", uuid.New().String()[:8], i),
			Settings:  map[string]interface{}{},
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := repo.Create(ctx, tenant); err != nil {
			t.Fatalf("Create tenant %d: %v", i, err)
		}
	}

	tenants, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(tenants) < 3 {
		t.Errorf("expected at least 3 tenants, got %d", len(tenants))
	}

	for _, tenant := range tenants {
		if tenant.Name == "" {
			t.Error("tenant name must not be empty")
		}
	}
}
