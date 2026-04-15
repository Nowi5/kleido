package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"kleido/internal/auth"
	"kleido/internal/config"
	"kleido/internal/handler"
	"kleido/internal/middleware"
	"kleido/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func buildRouter(
	pool *pgxpool.Pool,
	rdb *redis.Client,
	cfg *config.Config,
	log *slog.Logger,
	jwtSvc *auth.JWTService,
	authSvc service.AuthService,
	userSvc service.UserService,
	sessions middleware.SessionChecker,
	limiter middleware.RateLimiter,
) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware stack (order matters) ────────────────────────────────
	r.Use(middleware.Tracing(cfg.App.ServiceName)) // outermost: creates root OTel span
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RecoveringPanicCounter()) // replaces chimiddleware.Recoverer
	r.Use(middleware.SecurityHeaders(cfg.App.Env == "production"))
	r.Use(middleware.HTTPMetrics()) // must come after RecoveringPanicCounter
	r.Use(middleware.RequestLogger(log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── Metrics endpoint — no auth, no rate limit, not in Swagger ─────────────
	r.Handle("/metrics", promhttp.Handler())

	// ── Health probes (no auth, no rate limit) ─────────────────────────────────
	r.Get("/healthz", handler.Healthz())
	r.Get("/readyz", handler.Readyz(pool, rdb))

	// ── Swagger UI — development only ─────────────────────────────────────────
	if cfg.App.Env != "production" {
		registerSwaggerRoutes(r)
	}

	// ── API v1 ────────────────────────────────────────────────────────────────
	authH := handler.NewAuthHandler(authSvc, jwtSvc, cfg.App.Env == "production", cfg.App.RegistrationEnabled)
	userH := handler.NewUserHandler(userSvc)

	// sessionRepo implements both RateLimiter and UserRateLimiter interfaces.
	userLimiter, _ := limiter.(middleware.UserRateLimiter)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints — no JWT required.
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)
		r.Post("/auth/forgot-password", authH.ForgotPassword)
		r.Post("/auth/reset-password", authH.ResetPassword)

		// Protected endpoints — JWT + per-IP + per-user rate limits.
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWT(jwtSvc, sessions))
			r.Use(middleware.RateLimit(limiter, 100, time.Minute)) // per-IP

			// Per-user rate limit only when sessionRepo implements the interface.
			if userLimiter != nil {
				r.Use(middleware.RateLimitUser(userLimiter, 200, time.Minute))
			}

			r.Post("/auth/logout", authH.Logout)

			// Current user (any authenticated caller).
			r.Get("/users/me", userH.GetMe)

			// User management (access control enforced inside handler/service).
			r.Get("/users", userH.ListUsers)
			r.Get("/users/{id}", userH.GetUser)
			r.Put("/users/{id}", userH.UpdateUser)

			// Admin-only routes (enforced by middleware AND handler).
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Delete("/users/{id}", userH.DeleteUser)
			})
		})
	})

	// ── Admin panel (authenticated, admin role required) ───────────────────────
	// Registered before the SPA catch-all so /admin/* is not shadowed by /*.
	adminH := handler.NewAdminHandler(userSvc)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWT(jwtSvc, sessions))
		r.Use(middleware.RequireRole("admin"))
		r.Get("/admin/users", adminH.UserList)
		r.Delete("/admin/users/{id}", adminH.UserDelete)
	})

	// ── SPA + static assets — MUST be registered last ─────────────────────────
	// The /* catch-all must come after all API and admin routes.
	registerUIRoutes(r)

	return r
}
