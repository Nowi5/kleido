package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"kleido/internal/metrics"
	"kleido/internal/model"
	"kleido/internal/repository"
	"kleido/pkg/apperror"
)

// InstrumentedUserRepository wraps a UserRepository and records
// kleido_db_query_duration_seconds and kleido_db_errors_total for every call.
// apperror.IsNotFound errors are not counted as DB errors — they are expected
// business-logic outcomes (e.g. fetching a non-existent user).
type InstrumentedUserRepository struct {
	inner repository.UserRepository
}

// NewInstrumentedUserRepository wraps inner with Prometheus metrics instrumentation.
// Returns repository.UserRepository so callers remain interface-typed.
func NewInstrumentedUserRepository(inner repository.UserRepository) repository.UserRepository {
	return &InstrumentedUserRepository{inner: inner}
}

// Create records duration and errors for the Create operation.
func (r *InstrumentedUserRepository) Create(ctx context.Context, user *model.User) error {
	start := time.Now()
	err := r.inner.Create(ctx, user)
	metrics.DBQueryDurationSeconds.WithLabelValues("create").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBErrorsTotal.WithLabelValues("create").Inc()
	}
	return err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// FindByID records duration and errors for the FindByID operation.
// NotFound is not counted as an error — it is a normal business outcome.
func (r *InstrumentedUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	start := time.Now()
	user, err := r.inner.FindByID(ctx, id)
	metrics.DBQueryDurationSeconds.WithLabelValues("find_by_id").Observe(time.Since(start).Seconds())
	if err != nil && !apperror.IsNotFound(err) {
		metrics.DBErrorsTotal.WithLabelValues("find_by_id").Inc()
	}
	return user, err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// FindByEmail records duration and errors for the FindByEmail operation.
// NotFound is not counted as an error — it is a normal business outcome.
func (r *InstrumentedUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	start := time.Now()
	user, err := r.inner.FindByEmail(ctx, email)
	metrics.DBQueryDurationSeconds.WithLabelValues("find_by_email").Observe(time.Since(start).Seconds())
	if err != nil && !apperror.IsNotFound(err) {
		metrics.DBErrorsTotal.WithLabelValues("find_by_email").Inc()
	}
	return user, err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// Update records duration and errors for the Update operation.
func (r *InstrumentedUserRepository) Update(ctx context.Context, user *model.User) error {
	start := time.Now()
	err := r.inner.Update(ctx, user)
	metrics.DBQueryDurationSeconds.WithLabelValues("update").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBErrorsTotal.WithLabelValues("update").Inc()
	}
	return err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// UpdatePassword records duration and errors for the UpdatePassword operation.
func (r *InstrumentedUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	start := time.Now()
	err := r.inner.UpdatePassword(ctx, id, passwordHash)
	metrics.DBQueryDurationSeconds.WithLabelValues("update_password").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBErrorsTotal.WithLabelValues("update_password").Inc()
	}
	return err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// Delete records duration and errors for the Delete operation.
func (r *InstrumentedUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	err := r.inner.Delete(ctx, id)
	metrics.DBQueryDurationSeconds.WithLabelValues("delete").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBErrorsTotal.WithLabelValues("delete").Inc()
	}
	return err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}

// List records duration and errors for the List operation.
func (r *InstrumentedUserRepository) List(ctx context.Context, limit, offset int) ([]*model.User, int64, error) {
	start := time.Now()
	users, total, err := r.inner.List(ctx, limit, offset)
	metrics.DBQueryDurationSeconds.WithLabelValues("list").Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.DBErrorsTotal.WithLabelValues("list").Inc()
	}
	return users, total, err //nolint:wrapcheck // transparent passthrough — wrapping loses apperror type
}
