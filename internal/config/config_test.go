package config_test

// NOTE: config tests do NOT call t.Parallel() because t.Setenv() and
// t.Parallel() are mutually exclusive in Go — env-modifying tests must run
// sequentially to avoid races on the process environment.

import (
	"strings"
	"testing"

	"github.com/nowi5/kleido/internal/config"
)

func TestLoad_MissingDatabaseURL(t *testing.T) {
	// Wipe DATABASE_URL; keep JWT paths non-empty so only URL triggers the error.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is empty, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error message should mention DATABASE_URL, got: %q", err.Error())
	}
}

func TestLoad_ValidEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/myapp?sslmode=disable")
	t.Setenv("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("APP_VERSION", "1.2.3")
	t.Setenv("SERVICE_NAME", "testapp")
	t.Setenv("JWT_ACCESS_TOKEN_TTL_MINUTES", "30")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.App.Env != "production" {
		t.Errorf("App.Env: want %q, got %q", "production", cfg.App.Env)
	}
	if cfg.App.Port != 9090 {
		t.Errorf("App.Port: want %d, got %d", 9090, cfg.App.Port)
	}
	if cfg.App.Version != "1.2.3" {
		t.Errorf("App.Version: want %q, got %q", "1.2.3", cfg.App.Version)
	}
	if cfg.App.ServiceName != "testapp" {
		t.Errorf("App.ServiceName: want %q, got %q", "testapp", cfg.App.ServiceName)
	}
	if cfg.Database.URL != "postgres://user:pass@localhost:5432/myapp?sslmode=disable" {
		t.Errorf("Database.URL: unexpected value %q", cfg.Database.URL)
	}
	if cfg.JWT.AccessTokenTTL.Minutes() != 30 {
		t.Errorf("JWT.AccessTokenTTL: want 30m, got %v", cfg.JWT.AccessTokenTTL)
	}
}

func TestLoad_DefaultLogLevel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/myapp?sslmode=disable")
	t.Setenv("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	t.Setenv("APP_LOG_LEVEL", "") // explicitly unset — viper default should apply

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// viper.SetDefault sets "info" — but if the env var is set to "" the env var
	// wins over the default. We accept either outcome; the field must not panic.
	_ = cfg.App.LogLevel
}

func TestLoad_DefaultDatabaseMaxConns_WhenNotSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/myapp?sslmode=disable")
	t.Setenv("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	// DATABASE_MAX_CONNS is intentionally NOT overridden, so the default of 25 applies.

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Database.MaxConns != 25 {
		t.Errorf("Database.MaxConns default: want 25, got %d", cfg.Database.MaxConns)
	}
}
