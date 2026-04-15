package postgres

import (
	"context"

	"kleido/internal/model"
	"kleido/internal/repository"
	"kleido/pkg/apperror"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var repoTracer = otel.Tracer("kleido/repository")

// TracedUserRepository wraps a UserRepository and creates a child OTel span
// for every database operation. It uses the decorator pattern — same as
// InstrumentedUserRepository for Prometheus metrics.
type TracedUserRepository struct {
	inner repository.UserRepository
}

// NewTracedUserRepository wraps inner with OTel trace instrumentation.
// Returns repository.UserRepository so callers stay interface-typed.
func NewTracedUserRepository(inner repository.UserRepository) repository.UserRepository {
	return &TracedUserRepository{inner: inner}
}

func (r *TracedUserRepository) Create(ctx context.Context, user *model.User) error {
	ctx, span := repoTracer.Start(ctx, "repository/Create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("user.id", user.ID.String()),
	)

	err := r.inner.Create(ctx, user)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *TracedUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	ctx, span := repoTracer.Start(ctx, "repository/FindByID")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", id.String()),
	)

	user, err := r.inner.FindByID(ctx, id)
	if err != nil {
		if !apperror.IsNotFound(err) {
			// Not-found is an expected outcome, not a DB error — don't mark span as error.
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return user, nil
}

func (r *TracedUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	ctx, span := repoTracer.Start(ctx, "repository/FindByEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		// email is PII — not recorded as a span attribute
	)

	user, err := r.inner.FindByEmail(ctx, email)
	if err != nil {
		if !apperror.IsNotFound(err) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return user, nil
}

func (r *TracedUserRepository) Update(ctx context.Context, user *model.User) error {
	ctx, span := repoTracer.Start(ctx, "repository/Update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", user.ID.String()),
	)

	err := r.inner.Update(ctx, user)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *TracedUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	ctx, span := repoTracer.Start(ctx, "repository/UpdatePassword")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("user.id", id.String()),
	)

	err := r.inner.UpdatePassword(ctx, id, passwordHash)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *TracedUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := repoTracer.Start(ctx, "repository/Delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"), // soft delete is an UPDATE
		attribute.String("user.id", id.String()),
	)

	err := r.inner.Delete(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *TracedUserRepository) List(ctx context.Context, limit, offset int) ([]*model.User, int64, error) {
	ctx, span := repoTracer.Start(ctx, "repository/List")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
	)

	users, total, err := r.inner.List(ctx, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, err
	}
	span.SetStatus(codes.Ok, "")
	return users, total, nil
}
