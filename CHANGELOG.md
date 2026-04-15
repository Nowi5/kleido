# Changelog

All notable changes to kleido are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Sprint 10] — 2026-04-14

### Added

- **Brute-force login lockout** — 10 consecutive failures per email within 15 minutes returns HTTP 429; counter clears on successful login; Redis key uses SHA-256 of email (no PII in key names); fails open when Redis is unreachable
- **Per-user rate limiting middleware** — `RateLimitUser` middleware using chi route pattern as the rate limit key; wired into all authenticated routes after the per-IP limiter
- **Structured auth audit log** — every auth event emits a `log/slog` record with a fixed vocabulary of 7 `event_type` values, masked email (`u***@e***.com`), IP, user agent, and user ID
- **Password reset flow** — `POST /api/v1/auth/forgot-password` and `POST /api/v1/auth/reset-password`; single-use tokens stored SHA-256-hashed in Redis with 1-hour TTL; enumeration-safe (always returns 200)
- **`EmailSender` interface** with `StubEmailSender` that logs the reset URL via slog
- **`internal/reqctx` package** — propagates IP and user-agent through `context.Context`
- **`UserService.UpdatePassword`** — updates bcrypt hash and invalidates cache
- **SECURITY.md** — full threat model, auth model table, rate limiting details, audit log vocabulary, known limitations, incident response playbooks
- **RUNBOOK.md** — 7 operational playbooks covering 500s, login lockouts, DB/Redis loss, high latency, Prometheus scraping, and Grafana no-data

### Changed

- `NewAuthService` takes two new parameters: `mailer EmailSender` and `appBaseURL string`
- `SessionRepository` interface extended with 7 new methods: `IncrLoginFailure`, `GetLoginFailures`, `ClearLoginFailures`, `IsLockedOut`, `RateLimitAllowUser`, `StorePasswordResetToken`, `ConsumePasswordResetToken`
- `RateLimitAllow` refactored to delegate to shared `rateLimitByKey` helper; `RateLimitAllowUser` added alongside it
- Logger middleware now injects IP and user-agent into context via `reqctx`

---

## [Sprint 9] — 2026-03-31

### Added

- **Multi-stage Dockerfile** — Node → Go → `alpine:3.20` runtime; non-root user 1001; statically linked binary; `--build-arg VERSION` wired to `make docker-build`
- **`.dockerignore`** — excludes `.git`, `node_modules`, `keys/`, `bin/`, test files, docs
- **`docker-compose.yml`** — full 6-service stack with healthchecks, `restart: unless-stopped`, named volumes, and `kleido` bridge network
- **`docker-compose.override.yml`** — dev overrides (`APP_ENV: development`, `OTEL_ENABLED: false`, volume-mount for live reload)
- **GitHub Actions CI/CD pipeline** — 7 jobs: lint, swagger-check, build, test (matrix: unit + integration), security (govulncheck + Trivy), docker (push to ghcr.io on `main`)
- **Trivy container image scanning** — SARIF output uploaded to GitHub Security tab; HIGH/CRITICAL findings are blocking
- **`govulncheck`** — added to `tools.go` and Makefile `vuln-check` target; runs in CI security job
- **htmx Bearer token injection** — `<meta name="api-token">` in admin layout; JavaScript binds the token to all htmx requests as `Authorization: Bearer`; token extracted from `Authorization` header in `UserList` handler

### Changed

- `web/components/layout.templ` — `Layout(title, token string)` signature; meta tag and JS binding added
- `web/components/admin_users.templ` — `AdminUsers(users, total, token string)` signature
- Admin handler passes Bearer token from incoming request to templates

---

## [Sprint 8] — 2026-03-17

### Added

- **Admin panel** — `GET /admin/users` and `DELETE /admin/users/{id}` serve a server-rendered user list using `templ` v0.2.778 + htmx; partial rendering for htmx requests (no `<html>` wrapper)
- **`templ` code generation** — `make templ-generate` target; generated `*_templ.go` files committed

### Changed

- `AdminHandler.UserList` renders full page for regular requests, htmx partial for `HX-Request: true` requests
- `AdminHandler.UserDelete` returns empty 200 body (htmx swap removes the row)

---

## [Sprint 7] — 2026-03-10

### Added

- **CLI** — Cobra-based CLI with `auth login`, `auth logout`, `users list`, `users get`, `version`, and `completion` commands
- **`internal/client`** — typed Go API client used by the CLI; reads credentials from the configstore
- **`pkg/configstore`** — XDG-compliant credential store (access token + refresh token persisted per profile)

---

## [Sprint 6] — 2026-03-03

### Added

- **Distributed tracing** — OpenTelemetry SDK with OTLP gRPC exporter to Jaeger; all handler → service → repository calls are instrumented with child spans
- **Log–trace correlation** — `trace_id` and `span_id` injected into every slog record inside a traced request via `telemetry.NewSlogHandler`
- **`OTEL_ENABLED` flag** — set to `false` to disable tracing; all instrumentation becomes a no-op

---

## [Sprint 5] — 2026-02-24

### Added

- **Prometheus metrics** — `kleido_http_requests_total`, `kleido_http_request_duration_seconds`, `kleido_http_requests_inflight`, `kleido_http_panics_total`, `kleido_db_query_duration_seconds`, `kleido_db_errors_total`, `kleido_cache_operations_total`
- **`InstrumentedUserRepository`** — wraps the pgx repository; records query duration and error count
- **`TracedUserRepository`** — wraps the instrumented repository; records OTel spans
- **Grafana dashboard** — auto-provisioned "Kleido — API Overview" with 8 panels; datasource provisioned from `docker/grafana/`
- **`config/prometheus.yml`** — scrape config targeting `api:8080/metrics`

---

## [Sprint 4] — 2026-02-17

### Added

- **Redis sliding-window rate limiter** — 100 requests/minute/IP on all authenticated endpoints; sorted-set implementation via `ZADD` + `ZREMRANGEBYSCORE` + `ZCARD`
- **Security headers middleware** — `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, `Referrer-Policy`, `Permissions-Policy`, `Content-Security-Policy`, `HSTS` (production only)
- **Panic recovery middleware** — catches panics, logs stack trace, returns 500
- **Cache read-through** — `UserService.GetByID` checks Redis before hitting Postgres; `CacheRepository` interface backed by go-redis

### Changed

- `SessionRepository` — added `RateLimitAllow` method
- `middleware.JWT` — extracts JTI and checks Redis blocklist on every request

---

## [Sprint 3] — 2026-02-10

### Added

- **Token refresh** — `POST /api/v1/auth/refresh` reads refresh token from httpOnly cookie, validates via Redis, issues new access token, rotates refresh token
- **Logout** — `POST /api/v1/auth/logout` adds JTI to Redis blocklist, deletes refresh token from Redis, clears cookie
- **`golangci-lint` config** — `.golangci.yml` with gosec, staticcheck, errcheck, revive; `make lint` and `make lint-fix`
- **`SECURITY.md`** initial version

### Changed

- `auth.JWTService` — `Parse` returns JTI; `SignWithJTI` stores JTI in Redis on issue
- `service.AuthService.Login` — stores refresh token in Redis with TTL

---

## [Sprint 2] — 2026-02-03

### Added

- **User CRUD** — `GET /api/v1/users/me`, `GET /api/v1/users`, `GET /api/v1/users/{id}`, `PUT /api/v1/users/{id}`, `DELETE /api/v1/users/{id}`
- **JWT authentication middleware** — validates RS256 access token; injects `userID` and `role` into context
- **Role-based access control** — `RequireRole("admin")` middleware; non-admin users can only access their own profile
- **`pkg/apperror`** — typed `AppError` with HTTP status; `WriteError` writes consistent JSON error responses
- **`model.UpdateUserRequest`** — partial update with optional fields; `validate:"omitempty"` tags

### Changed

- `UserRepository` — added `List`, `Update`, `Delete` methods
- `UserService` — added `List`, `Update`, `Delete`, `GetByEmail` methods

---

## [Sprint 1] — 2026-01-27

### Added

- **Project scaffold** — Go module `github.com/nowi5/kleido`, layered architecture (handler → service → repository), `cmd/api/` entry point
- **PostgreSQL** — pgx v5 connection pool; `UserRepository` with `Create` and `GetByID`; golang-migrate SQL migrations
- **Registration** — `POST /api/v1/auth/register`; bcrypt password hashing; duplicate email returns 409
- **Login** — `POST /api/v1/auth/login`; RS256 JWT access token in response body; refresh token in httpOnly cookie
- **`auth.JWTService`** — RSA key loading from PEM files; `Sign` and `Parse` with RS256
- **Health probes** — `GET /healthz` (liveness) and `GET /readyz` (readiness; pings DB and Redis)
- **Structured logging** — `log/slog` JSON handler; secret redaction; `logger.FromContext`
- **Swagger UI** — swag annotations; `GET /swagger/*` disabled in production
- **`Makefile`** — `build`, `test`, `lint`, `swagger`, `migrate-*`, `gen-keys` targets
- **`.env.example`** — all configuration env vars with safe defaults
- **`scripts/gen-keys.sh`** — RSA-4096 keypair generator
