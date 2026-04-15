package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"kleido/internal/config"
	"kleido/internal/model"
	"kleido/internal/service"
	"kleido/pkg/apperror"

	"github.com/google/uuid"
)

// seedAdminUser ensures at least one admin account exists in the database.
// It is idempotent: if an admin with the configured email already exists the
// function returns immediately without modifying anything.
//
// Credentials are controlled via SEED_ADMIN_EMAIL / SEED_ADMIN_PASSWORD.
// Override these in docker-compose or your .env file for any environment.
func seedAdminUser(ctx context.Context, cfg config.SeedConfig, userSvc service.UserService, log *slog.Logger) {
	email := cfg.AdminEmail
	password := cfg.AdminPassword

	// Skip if already present.
	existing, err := userSvc.GetByEmail(ctx, email)
	if err == nil && existing != nil {
		log.Info("seed: admin user already exists, skipping", slog.String("email", email))
		return
	}

	// Any error other than "not found" is unexpected — log and bail out.
	var appErr *apperror.AppError
	if err != nil && (!errors.As(err, &appErr) || appErr.Code != 404) {
		log.Error("seed: failed to check for existing admin user", slog.Any("error", err))
		return
	}

	user, err := userSvc.Create(ctx, email, password, "admin")
	if err != nil {
		log.Error("seed: failed to create admin user", slog.Any("error", err))
		return
	}

	log.Info("seed: admin user created",
		slog.String("email", user.Email),
		slog.String("user_id", user.ID.String()),
	)
}

// seedDefaultTenant ensures a default tenant exists in the database.
// It is idempotent: if a tenant with slug "default" already exists,
// the function returns immediately without modifying anything.
func seedDefaultTenant(ctx context.Context, tenantSvc service.TenantService, log *slog.Logger) {
	existing, err := tenantSvc.GetBySlug(ctx, "default")
	if err == nil && existing != nil {
		log.Info("seed: default tenant already exists, skipping")
		return
	}

	var appErr *apperror.AppError
	if err != nil && (!errors.As(err, &appErr) || appErr.Code != 404) {
		log.Error("seed: failed to check for existing tenant", slog.Any("error", err))
		return
	}

	tenant := &model.Tenant{
		ID:        uuid.New(),
		Name:      "Default Organization",
		Slug:      "default",
		Settings:  map[string]interface{}{},
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := tenantSvc.Create(ctx, tenant); err != nil {
		log.Error("seed: failed to create default tenant", slog.Any("error", err))
		return
	}

	log.Info("seed: default tenant created",
		slog.String("tenant_id", tenant.ID.String()),
		slog.String("slug", tenant.Slug),
	)
}
