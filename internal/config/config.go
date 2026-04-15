// Package config loads and validates application configuration from environment
// variables and an optional .env file using Viper.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the top-level application configuration.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	API      APIConfig
	OTel     OTelConfig
}

// OTelConfig holds OpenTelemetry tracing settings (Sprint 6).
type OTelConfig struct {
	Enabled  bool
	Endpoint string // host:port for the OTLP gRPC exporter, no scheme prefix
}

// AppConfig holds general application settings.
type AppConfig struct {
	Env                 string
	Port                int
	LogLevel            string
	Version             string
	ServiceName         string
	RegistrationEnabled bool
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL                    string
	MaxConns               int32
	MinConns               int32
	MaxConnLifetimeMinutes int
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

// JWTConfig holds JWT settings.
type JWTConfig struct {
	PrivateKeyPath      string
	PublicKeyPath       string
	AccessTokenTTL      time.Duration
	RefreshTokenTTLDays int
}

// APIConfig holds the API client settings (used by CLI).
type APIConfig struct {
	BaseURL string
}

// Load reads configuration from the environment (and optional .env file)
// and returns a fully-populated *Config. It returns an error if any required
// field is missing.
func Load() (*Config, error) {
	v := viper.New()

	// Read optional .env file; ignore if absent.
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig() //nolint:errcheck // .env is optional; absence is not an error

	// Environment variables take precedence over the .env file.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults — App.
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("APP_LOG_LEVEL", "info")
	v.SetDefault("APP_VERSION", "0.1.0")
	v.SetDefault("SERVICE_NAME", "kleido")
	v.SetDefault("AUTH_REGISTRATION_ENABLED", true)

	// Defaults — Database.
	v.SetDefault("DATABASE_MAX_CONNS", 25)
	v.SetDefault("DATABASE_MIN_CONNS", 5)
	v.SetDefault("DATABASE_MAX_CONN_LIFETIME_MINUTES", 30)

	// Defaults — Redis.
	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_POOL_SIZE", 20)

	// Defaults — JWT.
	v.SetDefault("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	v.SetDefault("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	v.SetDefault("JWT_ACCESS_TOKEN_TTL_MINUTES", 15)
	v.SetDefault("JWT_REFRESH_TOKEN_TTL_DAYS", 7)

	// Defaults — API.
	v.SetDefault("API_BASE_URL", "http://localhost:8080")

	// Defaults — OpenTelemetry (Sprint 6).
	v.SetDefault("OTEL_ENABLED", true)
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	// Validate required fields.
	var errs []string

	databaseURL := v.GetString("DATABASE_URL")
	if databaseURL == "" {
		errs = append(errs, "DATABASE_URL is required but not set")
	}

	jwtPrivatePath := v.GetString("JWT_PRIVATE_KEY_PATH")
	if jwtPrivatePath == "" {
		errs = append(errs, "JWT_PRIVATE_KEY_PATH is required but not set")
	}

	jwtPublicPath := v.GetString("JWT_PUBLIC_KEY_PATH")
	if jwtPublicPath == "" {
		errs = append(errs, "JWT_PUBLIC_KEY_PATH is required but not set")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("config: %s", strings.Join(errs, "; "))
	}

	accessTTL := time.Duration(v.GetInt("JWT_ACCESS_TOKEN_TTL_MINUTES")) * time.Minute

	cfg := &Config{
		App: AppConfig{
			Env:                 v.GetString("APP_ENV"),
			Port:                v.GetInt("APP_PORT"),
			LogLevel:            v.GetString("APP_LOG_LEVEL"),
			Version:             v.GetString("APP_VERSION"),
			ServiceName:         v.GetString("SERVICE_NAME"),
			RegistrationEnabled: v.GetBool("AUTH_REGISTRATION_ENABLED"),
		},
		Database: DatabaseConfig{
			URL:                    databaseURL,
			MaxConns:               int32(v.GetInt("DATABASE_MAX_CONNS")), //nolint:gosec // value is bounded by sane config
			MinConns:               int32(v.GetInt("DATABASE_MIN_CONNS")), //nolint:gosec // value is bounded by sane config
			MaxConnLifetimeMinutes: v.GetInt("DATABASE_MAX_CONN_LIFETIME_MINUTES"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("REDIS_ADDR"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
			PoolSize: v.GetInt("REDIS_POOL_SIZE"),
		},
		JWT: JWTConfig{
			PrivateKeyPath:      jwtPrivatePath,
			PublicKeyPath:       jwtPublicPath,
			AccessTokenTTL:      accessTTL,
			RefreshTokenTTLDays: v.GetInt("JWT_REFRESH_TOKEN_TTL_DAYS"),
		},
		API: APIConfig{
			BaseURL: v.GetString("API_BASE_URL"),
		},
		OTel: OTelConfig{
			Enabled:  v.GetBool("OTEL_ENABLED"),
			Endpoint: v.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
		},
	}

	return cfg, nil
}
