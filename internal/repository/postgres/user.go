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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// userRepository is the pgx-backed implementation of repository.UserRepository.
// It is intentionally unexported; callers receive the interface.
type userRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository returns a repository.UserRepository backed by the given pool.
func NewUserRepository(pool *pgxpool.Pool) repository.UserRepository {
	return &userRepository{pool: pool}
}

// Create inserts a new user row. The user.ID must already be set by the caller.
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	const q = `
		INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, q,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("email already registered")
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindByID fetches an active user by primary key.
// Returns apperror.NotFound("user") when the row does not exist.
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	const q = `
		SELECT id, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1 AND is_active = true`

	row := r.pool.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("user")
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

// FindByEmail fetches a user by email address (active or inactive — callers
// need to inspect IsActive for login flows).
// Returns apperror.NotFound("user") when the row does not exist.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `
		SELECT id, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE email = $1`

	row := r.pool.QueryRow(ctx, q, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("user")
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

// Update writes email, role, is_active, and updated_at for the given user.
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	const q = `
		UPDATE users
		SET email = $1, role = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4`

	_, err := r.pool.Exec(ctx, q, user.Email, user.Role, user.IsActive, user.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// UpdatePassword sets a new password_hash for the user identified by id.
// No other fields are modified.
func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	const q = `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`

	_, err := r.pool.Exec(ctx, q, passwordHash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// Delete performs a soft delete by setting is_active = false.
// The row is never physically removed.
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// List returns a page of users ordered by created_at DESC, along with the total
// count of all users (for pagination metadata).
func (r *userRepository) List(ctx context.Context, limit, offset int) ([]*model.User, int64, error) {
	// Total count query.
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("list users count: %w", err)
	}

	// Data query — omit password_hash for safety; reconstruct the full struct
	// with an empty hash so callers can still use model.User.
	const q = `
		SELECT id, email, role, is_active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users query: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.Role,
			&u.IsActive,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("list users scan: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list users rows: %w", err)
	}

	return users, total, nil
}

// scanUser reads a single user row from a pgx.Row.
func scanUser(row pgx.Row) (*model.User, error) {
	u := &model.User{}
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: scan user: %w", err)
	}
	return u, nil
}
