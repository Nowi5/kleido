package postgres

import (
	"context"
	"errors"
	"fmt"

	"kleido/internal/model"
	"kleido/internal/repository"
	"kleido/pkg/apperror"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) repository.TenantRepository {
	return &tenantRepository{pool: pool}
}

func (r *tenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	const q = `
		INSERT INTO tenants (id, name, slug, settings, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, q,
		tenant.ID,
		tenant.Name,
		tenant.Slug,
		tenant.Settings,
		tenant.IsActive,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (r *tenantRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	const q = `
		SELECT id, name, slug, settings, is_active, created_at, updated_at
		FROM tenants
		WHERE id = $1 AND is_active = true`

	row := r.pool.QueryRow(ctx, q, id)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("tenant")
		}
		return nil, fmt.Errorf("find tenant by id: %w", err)
	}
	return t, nil
}

func (r *tenantRepository) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	const q = `
		SELECT id, name, slug, settings, is_active, created_at, updated_at
		FROM tenants
		WHERE slug = $1 AND is_active = true`

	row := r.pool.QueryRow(ctx, q, slug)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("tenant")
		}
		return nil, fmt.Errorf("find tenant by slug: %w", err)
	}
	return t, nil
}

func (r *tenantRepository) List(ctx context.Context) ([]*model.Tenant, error) {
	const q = `
		SELECT id, name, slug, settings, is_active, created_at, updated_at
		FROM tenants
		WHERE is_active = true
		ORDER BY name ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*model.Tenant
	for rows.Next() {
		t := &model.Tenant{}
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Slug,
			&t.Settings,
			&t.IsActive,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tenants rows: %w", err)
	}

	return tenants, nil
}

func scanTenant(row pgx.Row) (*model.Tenant, error) {
	t := &model.Tenant{}
	err := row.Scan(
		&t.ID,
		&t.Name,
		&t.Slug,
		&t.Settings,
		&t.IsActive,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: scan tenant: %w", err)
	}
	return t, nil
}