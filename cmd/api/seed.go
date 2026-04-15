package main

import (
	"context"
	"errors"
	"log/slog"

	"kleido/internal/config"
	"kleido/internal/service"
	"kleido/pkg/apperror"
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
