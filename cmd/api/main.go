// Package main is the entry point for the kleido API server.
//
//	@title						Kleido API
//	@version					1.0
//	@description				Production-grade Go REST API. JWT RS256 authentication required on all /api/v1/ endpoints except /auth/register and /auth/login.
//
//	@contact.name				API Support
//	@contact.url				https://github.com/nowi5/kleido
//	@contact.email				api@example.com
//
//	@license.name				MIT
//	@license.url				https://opensource.org/licenses/MIT
//
//	@host						localhost:8080
//	@BasePath					/api/v1
//	@schemes					http https
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and the JWT access token. Example: "Bearer eyJhbGci..."
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kleido/internal/auth"
	"kleido/internal/config"
	"kleido/internal/logger"
	repopostgres "kleido/internal/repository/postgres"
	reporedis "kleido/internal/repository/redis"
	"kleido/internal/service"
	"kleido/internal/telemetry"
)

// version is injected at build time via -ldflags.
var version = "dev"

func run() error {
	// 1. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Initialize structured logger.
	log := logger.New(cfg.App.LogLevel, cfg.App.Env, cfg.App.ServiceName, version)
	slog.SetDefault(log)

	// 2a. Background context used for OTel setup and pool creation.
	ctx := context.Background()

	// 2b. Initialise OpenTelemetry TracerProvider and wire shutdown.
	otelShutdown, err := telemetry.Setup(
		ctx,
		cfg.App.ServiceName,
		cfg.App.Version,
		cfg.OTel.Endpoint,
		cfg.App.Env,
		cfg.OTel.Enabled,
	)
	if err != nil {
		return fmt.Errorf("setup OTel: %w", err)
	}
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			log.Error("OTel shutdown error", slog.Any("error", shutdownErr))
		}
	}()

	// Rebuild logger with OTel bridge so trace_id appears in JSON logs.
	// Only wrap when OTel is enabled — avoids overhead when tracing is off.
	if cfg.OTel.Enabled {
		log = slog.New(telemetry.NewSlogHandler(log.Handler(), cfg.App.ServiceName))
		slog.SetDefault(log)
	}

	// 3. Run database migrations.
	if migErr := repopostgres.RunMigrations(cfg.Database.URL, "./migrations"); migErr != nil {
		return fmt.Errorf("migrations: %w", migErr)
	}

	// 4. Create PostgreSQL connection pool.
	pool, poolErr := repopostgres.NewPool(ctx, cfg.Database)
	if poolErr != nil {
		return fmt.Errorf("postgres pool: %w", poolErr)
	}
	defer pool.Close()

	// 5. Create Redis client.
	rdb, redisErr := reporedis.NewClient(cfg.Redis)
	if redisErr != nil {
		return fmt.Errorf("redis client: %w", redisErr)
	}
	defer func() {
		if closeErr := rdb.Close(); closeErr != nil {
			log.Warn("redis close error", slog.Any("error", closeErr))
		}
	}()

	// 6. Load RSA keys.
	privKey, privErr := auth.LoadPrivateKey(cfg.JWT.PrivateKeyPath)
	if privErr != nil {
		return fmt.Errorf("jwt private key: %w", privErr)
	}
	pubKey, pubErr := auth.LoadPublicKey(cfg.JWT.PublicKeyPath)
	if pubErr != nil {
		return fmt.Errorf("jwt public key: %w", pubErr)
	}

	// 7. Construct repositories.
	// Chain: Tracing (outermost) → Metrics → pgx implementation (innermost).
	userRepo := repopostgres.NewTracedUserRepository(
		repopostgres.NewInstrumentedUserRepository(
			repopostgres.NewUserRepository(pool),
		),
	)
	sessionRepo := reporedis.NewSessionRepo(rdb)
	cacheRepo := reporedis.NewCacheRepo(rdb)

	// 8. Construct JWT service.
	jwtSvc := auth.NewJWTService(privKey, pubKey, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTLDays)

	// 9. Construct application services.
	userSvc := service.NewUserService(userRepo, cacheRepo, log)
	authSvc := service.NewAuthService(userSvc, sessionRepo, jwtSvc, log,
		&service.StubEmailSender{}, cfg.API.BaseURL)

	// 10. Seed initial data (idempotent — skipped if admin already exists).
	seedAdminUser(ctx, cfg.Seed, userSvc, log)

	// 11. Build router (passes cfg so it can check APP_ENV for Swagger UI).
	router := buildRouter(pool, rdb, cfg, log, jwtSvc, authSvc, userSvc, sessionRepo, sessionRepo)

	// 12. Configure HTTP server.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	// 13. Start server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("server starting", slog.String("addr", srv.Addr), slog.String("env", cfg.App.Env))
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			serverErr <- listenErr
		}
	}()

	// 14. Block on OS signal or server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", slog.String("signal", sig.String()))
	case srvErr := <-serverErr:
		log.Error("server error", slog.Any("error", srvErr))
	}

	// 15. Graceful shutdown with 15-second timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Error("graceful shutdown failed", slog.Any("error", shutdownErr))
	} else {
		log.Info("server stopped")
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
}
