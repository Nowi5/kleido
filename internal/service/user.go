package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"kleido/internal/logger"
	"kleido/internal/metrics"
	"kleido/internal/model"
	"kleido/internal/repository"
	"kleido/pkg/apperror"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/crypto/bcrypt"
)

var userTracer = otel.Tracer("kleido/service")

const userCacheTTL = 5 * time.Minute

type userService struct {
	repo  repository.UserRepository
	cache repository.CacheRepository
	log   *slog.Logger
}

// NewUserService creates a UserService backed by a repository and an optional
// Redis cache. Pass nil for cache to disable caching (not recommended in prod).
func NewUserService(repo repository.UserRepository, cache repository.CacheRepository, log *slog.Logger) UserService {
	return &userService{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

func userCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("cache:user:%s", id)
}

// GetByID fetches a user by ID. It checks the cache first; on a miss it falls
// through to the database and repopulates the cache. Cache operations are
// recorded via kleido_cache_operations_total.
func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	ctx, span := userTracer.Start(ctx, "service/GetByID")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id.String()))

	log := logger.FromContext(ctx)
	key := userCacheKey(id)

	// Cache read-through.
	if s.cache != nil {
		var cached model.User
		cacheErr := s.cache.GetJSON(ctx, key, &cached)
		if cacheErr == nil {
			metrics.CacheOperationsTotal.WithLabelValues("get", "hit").Inc()
			return &cached, nil
		}
		if errors.Is(cacheErr, redis.Nil) {
			metrics.CacheOperationsTotal.WithLabelValues("get", "miss").Inc()
		} else {
			metrics.CacheOperationsTotal.WithLabelValues("get", "error").Inc()
			log.WarnContext(ctx, "cache get error", slog.String("key", key), slog.Any("error", cacheErr))
		}
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err //nolint:wrapcheck // apperror.NotFound from repo; wrapping loses type
	}

	// Populate cache.
	if s.cache != nil {
		if setErr := s.cache.SetJSON(ctx, key, user, userCacheTTL); setErr != nil {
			metrics.CacheOperationsTotal.WithLabelValues("set", "error").Inc()
			log.WarnContext(ctx, "cache set error", slog.String("key", key), slog.Any("error", setErr))
		} else {
			metrics.CacheOperationsTotal.WithLabelValues("set", "ok").Inc()
		}
	}

	span.SetStatus(codes.Ok, "")
	return user, nil
}

// GetByEmail fetches a user by email. No caching — auth flows need fresh data.
func (s *userService) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.repo.FindByEmail(ctx, email) //nolint:wrapcheck // apperror from repo; wrapping loses type
}

// Create hashes the password with bcrypt (cost 12), generates a UUID, and
// inserts a new user row via the repository.
func (s *userService) Create(ctx context.Context, email, password, role string) (*model.User, error) {
	ctx, span := userTracer.Start(ctx, "service/Create")
	defer span.End()

	log := logger.FromContext(ctx)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		appErr := apperror.Internal(fmt.Errorf("hash password: %w", err))
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		return nil, appErr
	}

	user := &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	span.SetAttributes(attribute.String("user.id", user.ID.String()))
	log.InfoContext(ctx, "user created", slog.String("user_id", user.ID.String()))
	span.SetStatus(codes.Ok, "")
	return user, nil
}

// Update applies a partial update (patch semantics) to the user identified by id.
// Role changes are silently ignored when callerRole is not "admin".
// The cache entry is invalidated before returning.
func (s *userService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateUserRequest, callerRole string) (*model.User, error) {
	ctx, span := userTracer.Start(ctx, "service/Update")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id.String()))

	log := logger.FromContext(ctx)

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Role != nil {
		if callerRole != "admin" {
			log.DebugContext(ctx, "role change ignored — caller is not admin", slog.String("user_id", id.String()))
		} else {
			user.Role = *req.Role
		}
	}

	if err := s.repo.Update(ctx, user); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	if s.cache != nil {
		if err := s.cache.Delete(ctx, userCacheKey(id)); err != nil {
			log.WarnContext(ctx, "cache delete failed", slog.String("key", userCacheKey(id)), slog.Any("error", err))
		}
	}

	span.SetStatus(codes.Ok, "")
	return user, nil
}

// Delete soft-deletes the user (sets is_active = false) and invalidates the cache.
// Returns apperror.NotFound if the user does not exist.
func (s *userService) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := userTracer.Start(ctx, "service/Delete")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id.String()))

	log := logger.FromContext(ctx)

	if _, err := s.repo.FindByID(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err //nolint:wrapcheck // apperror.NotFound from repo; wrapping loses type
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	if s.cache != nil {
		if err := s.cache.Delete(ctx, userCacheKey(id)); err != nil {
			log.WarnContext(ctx, "cache delete failed", slog.String("key", userCacheKey(id)), slog.Any("error", err))
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// UpdatePassword sets a new (already bcrypt-hashed) password for the user.
// Invalidates the cache entry so the next read fetches fresh data.
func (s *userService) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	ctx, span := userTracer.Start(ctx, "service/UpdatePassword")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id.String()))

	if err := s.repo.UpdatePassword(ctx, id, passwordHash); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	if s.cache != nil {
		if err := s.cache.Delete(ctx, userCacheKey(id)); err != nil {
			logger.FromContext(ctx).WarnContext(ctx, "cache delete failed after password update",
				slog.String("key", userCacheKey(id)), slog.Any("error", err))
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// List returns a page of safe UserResponse objects and the total count.
func (s *userService) List(ctx context.Context, limit, offset int) ([]*model.UserResponse, int64, error) {
	ctx, span := userTracer.Start(ctx, "service/List")
	defer span.End()
	span.SetAttributes(
		attribute.Int("pagination.per_page", limit),
	)

	users, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, err //nolint:wrapcheck // apperror from repo; wrapping loses type
	}

	responses := make([]*model.UserResponse, len(users))
	for i, u := range users {
		responses[i] = u.ToResponse()
	}
	span.SetStatus(codes.Ok, "")
	return responses, total, nil
}
