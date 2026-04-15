# Kleido

![CI](https://github.com/nowi5/kleido/actions/workflows/ci.yml/badge.svg)
[![Coverage](https://codecov.io/gh/nowi5/kleido/branch/main/graph/badge.svg)](https://codecov.io/gh/nowi5/kleido)
![Go Version](https://img.shields.io/badge/Go-1.25+-blue)
![License](https://img.shields.io/badge/license-MIT-green)

Kleido is a production-grade REST API backend written in Go — it provides secure user authentication (RS256 JWT, bcrypt, Redis session management), a clean layered architecture (handler → service → repository), and a full operational foundation (structured logging, health probes, graceful shutdown, golangci-lint, gosec).

## Architecture Overview

```
CLI ──────────────────────────────┐
                                  ▼
Browser / SPA  ──►  chi Router  ──►  Middleware Chain
                                  │   (JWT · CORS · RateLimit · SecurityHeaders · Logger)
                                  ▼
                            Handler Layer
                                  │
                                  ▼
                            Service Layer  ◄──  business logic, cache read-through
                                  │
                          ┌───────┴───────┐
                          ▼               ▼
                     PostgreSQL         Redis
                    (pgx v5)        (go-redis v9)
```

## Prerequisites

| Tool | Minimum version | Install |
|------|----------------|---------|
| Go | 1.25 | `brew install go` / [go.dev/dl](https://go.dev/dl) |
| Docker + Compose | 24.x | [docs.docker.com](https://docs.docker.com/get-docker/) |
| make | any | system package manager |
| golangci-lint | v1.59+ | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |

## Quick Start

```bash
# 1. Clone
git clone https://github.com/nowi5/kleido.git
cd kleido

# 2. Copy environment config
cp .env.example .env          # Linux / macOS / Git Bash
# copy .env.example .env      # Windows Command Prompt
# Copy-Item .env.example .env # Windows PowerShell

# 3. Generate RSA keys
bash scripts/gen-keys.sh      # Linux / macOS / Git Bash
# Windows (OpenSSL):
# mkdir keys
# openssl genrsa -out keys/private.pem 4096
# openssl rsa -in keys/private.pem -pubout -out keys/public.pem

# 4. Start full stack (API + Postgres + Redis + Jaeger + Prometheus + Grafana + Docs)
docker-compose up --build -d

# 5. Verify
curl http://localhost:8080/healthz   # → {"status":"ok"}
curl http://localhost:8080/readyz    # → {"status":"ready","db":true,"redis":true}
# Docs: http://localhost:8001
```

> **Windows note:** Use [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/) with the WSL 2 backend. If you see a `dockerDesktopLinuxEngine` error, run `docker context use default` first.

### Backing services only (local Go development)

```bash
# Start only the backing services, then run the API from source:
docker-compose up -d postgres redis jaeger
go run ./cmd/api/
```

## Available Make Targets

> **Windows note:** `make` targets require [GNU Make for Windows](https://gnuwin32.sourceforge.net/packages/make.htm) or Git Bash / WSL 2. Alternatively, use Docker (`docker-compose up --build -d`) as the primary workflow — no `make` needed.

| Target | Description | Example |
|--------|-------------|---------|
| `build` | Compile API and CLI binaries to `./bin/` | `make build` |
| `run` | Run the API server locally (use `go run ./cmd/api/` on Windows) | `make run` |
| `test` | Unit tests with race detector | `make test` |
| `test-integration` | Integration tests (requires Docker) | `make test-integration` |
| `test-coverage` | Coverage report → `coverage.html` | `make test-coverage` |
| `lint` | Run golangci-lint | `make lint` |
| `lint-fix` | Run golangci-lint with auto-fix | `make lint-fix` |
| `fmt` | Format all Go files with gofmt | `make fmt` |
| `check` | fmt + vet + lint + test + swagger-check | `make check` |
| `swagger` | Regenerate OpenAPI docs from annotations | `make swagger` |
| `swagger-check` | Verify docs are not stale (CI gate) | `make swagger-check` |
| `migrate-up` | Apply all pending migrations | `make migrate-up` |
| `migrate-down` | Roll back one migration | `make migrate-down` |
| `migrate-create NAME=x` | Create a new migration pair | `make migrate-create NAME=add_posts` |
| `gen-keys` | Generate RSA keypair to `./keys/` | `make gen-keys` |
| `generate` | Run mockery + templ + swag | `make generate` |
| `ui-build` | Build the frontend (`web/dist/`) | `make ui-build` |
| `docker-build` | Build production Docker image | `make docker-build` |
| `docker-up` | Start full stack via docker-compose | `make docker-up` |
| `docker-down` | Stop and remove containers | `make docker-down` |
| `clean` | Remove build artefacts | `make clean` |

## Project Structure

```
kleido/                         ← repo root (this directory)
├── cmd/
│   ├── api/                    ← HTTP server entry point (main.go + routes.go)
│   └── cli/                    ← CLI entry point
├── config/
│   └── prometheus.yml          ← Prometheus scrape config (scrapes api:8080/metrics)
├── docker/
│   └── grafana/
│       ├── dashboards/         ← Auto-provisioned Grafana dashboard JSON
│       └── provisioning/       ← Grafana datasource + dashboard provider YAMLs
├── internal/
│   ├── auth/                   ← RSA key loading + JWT sign/verify (RS256 only)
│   ├── client/                 ← Typed Go API client (used by CLI)
│   ├── cli/                    ← Cobra CLI commands (auth, users, version, completion)
│   ├── config/                 ← Typed viper config; only place os.Getenv is allowed
│   ├── handler/                ← HTTP handlers — decode request, call service, encode response
│   ├── logger/                 ← log/slog wrapper with secret redaction + context helpers
│   ├── metrics/                ← Prometheus metric definitions (promauto package-level vars)
│   ├── middleware/             ← JWT auth, rate limiting, security headers, metrics, panic recovery
│   ├── model/                  ← Domain structs (User, UserResponse)
│   ├── repository/
│   │   ├── interfaces.go       ← UserRepository, SessionRepository, CacheRepository
│   │   ├── postgres/           ← pgx v5 implementations + InstrumentedUserRepository
│   │   └── redis/              ← go-redis v9 implementations
│   └── service/                ← Business logic; imports repository interfaces only
├── migrations/                 ← golang-migrate SQL files
├── mkdocs/                     ← Developer documentation (MkDocs + Material)
│   ├── mkdocs.yml              ←   site config, navigation, theme
│   └── content/                ←   Markdown source pages
├── pkg/
│   ├── apperror/               ← Typed AppError + WriteError; no http.Error() elsewhere
│   └── configstore/            ← XDG-compliant CLI credential store
├── scripts/
│   └── gen-keys.sh             ← RSA-4096 keypair generator
├── docs/                       ← swag-generated OpenAPI package (committed, not edited by hand)
├── .env.example                ← All env vars with defaults
├── .golangci.yml               ← Linter configuration
├── docker-compose.yml          ← Full stack: API + Postgres + Redis + Jaeger + Prometheus + Grafana + Docs
└── Makefile                    ← All development targets
```

## Configuration Reference

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `APP_ENV` | `development` | No | Runtime environment. Affects log format and Swagger visibility. |
| `APP_PORT` | `8080` | No | HTTP listen port. |
| `APP_LOG_LEVEL` | `info` | No | `debug` \| `info` \| `warn` \| `error` |
| `APP_VERSION` | `dev` | No | Injected at build time via `-ldflags`. |
| `SERVICE_NAME` | `kleido` | No | Appears in every log entry. |
| `AUTH_REGISTRATION_ENABLED` | `true` | No | Set to `false` to disable the public `POST /api/v1/auth/register` endpoint. Useful for closed/invite-only deployments. Returns HTTP 403 when disabled. |
| `DATABASE_URL` | — | **Yes** | Full PostgreSQL DSN. |
| `DATABASE_MAX_CONNS` | `25` | No | pgxpool max connections. |
| `DATABASE_MIN_CONNS` | `5` | No | pgxpool min idle connections. |
| `DATABASE_MAX_CONN_LIFETIME_MINUTES` | `30` | No | pgxpool max connection lifetime. |
| `REDIS_ADDR` | `localhost:6379` | No | Redis host:port. |
| `REDIS_PASSWORD` | — | No | Redis AUTH password. |
| `REDIS_DB` | `0` | No | Redis logical database number. |
| `REDIS_POOL_SIZE` | `20` | No | go-redis pool size. |
| `JWT_PRIVATE_KEY_PATH` | — | **Yes** | Path to RSA private key PEM. Generate with `make gen-keys`. |
| `JWT_PUBLIC_KEY_PATH` | — | **Yes** | Path to RSA public key PEM. |
| `JWT_ACCESS_TOKEN_TTL_MINUTES` | `15` | No | Access token lifetime in minutes. |
| `JWT_REFRESH_TOKEN_TTL_DAYS` | `7` | No | Refresh token lifetime in days. |
| `API_BASE_URL` | `http://localhost:8080` | No | Used by CLI to locate the API. |

## API Overview

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | None | Liveness probe — always 200 if process is running |
| `GET` | `/readyz` | None | Readiness probe — 503 if DB or Redis is unreachable |
| `POST` | `/api/v1/auth/register` | None | Create new user account (returns 403 if `AUTH_REGISTRATION_ENABLED=false`) |
| `POST` | `/api/v1/auth/login` | None | Authenticate; returns access token + sets refresh cookie |
| `POST` | `/api/v1/auth/refresh` | Cookie | Issue new access token; rotates refresh token |
| `POST` | `/api/v1/auth/logout` | Bearer | Revoke current access token and refresh token |
| `POST` | `/api/v1/auth/forgot-password` | None | Request a password reset email (always returns 200) |
| `POST` | `/api/v1/auth/reset-password` | None | Reset password using a single-use token from email |
| `GET` | `/api/v1/users/me` | Bearer | Get the authenticated user's own profile |
| `GET` | `/api/v1/users` | Bearer (admin) | List all users, paginated |
| `GET` | `/api/v1/users/{id}` | Bearer | Get user by ID |
| `PUT` | `/api/v1/users/{id}` | Bearer | Partially update a user |
| `DELETE` | `/api/v1/users/{id}` | Bearer (admin) | Soft-delete a user |
| `GET` | `/admin/users` | Bearer (admin) | Admin panel — SSR user list (templ + htmx) |
| `DELETE` | `/admin/users/{id}` | Bearer (admin) | Admin panel — soft-delete; used by htmx |
| `GET` | `/` | None | React SPA entry point (`Cache-Control: no-store`) |
| `GET` | `/assets/*` | None | Hashed static assets (`Cache-Control: immutable`) |

## Web UI

The application ships two built-in browser interfaces, both served by the Go API process on port 8080.

### React SPA (user-facing)

The React single-page application is embedded in the Go binary and served at the root path.

**Open:** `http://localhost:8080/`

Once you have registered and logged in via the API (or the Swagger UI), the SPA shows your user profile by fetching `GET /api/v1/users/me`. It uses an in-memory access token (never written to `localStorage`) and an httpOnly refresh cookie for session persistence.

**Typical first-use flow:**

```bash
# 1. Register a new account (if AUTH_REGISTRATION_ENABLED=true)
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"Password1!"}' | jq .

# 2. Log in — the refresh cookie is set automatically by the browser
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"Password1!"}' | jq .access_token

# 3. Open http://localhost:8080 in your browser and supply the access token
#    when prompted (or use the Swagger UI at http://localhost:8080/swagger/index.html)
```

> **Dev mode:** Run `make ui-dev` to start the Vite dev server on port 5173 with hot-module replacement. All `/api/*` requests are proxied to port 8080. Keep the Go API running in a separate terminal (`make run`).

### Admin panel

A server-rendered admin interface built with `templ` + `htmx` is available at `/admin/users`. It requires an **admin** JWT and lists all users with soft-delete actions.

**Open:** `http://localhost:8080/admin/users` (must supply a valid admin Bearer token)

```bash
# Log in as admin and capture the access token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Password1!"}' | jq -r .access_token)

# Open the admin panel — pass the token as a query parameter or via browser storage
echo "http://localhost:8080/admin/users  (set Authorization: Bearer $TOKEN in your browser)"
```

> The admin panel uses `htmx` for partial page updates. Delete actions send `DELETE /admin/users/{id}` and remove the row inline without a full page reload.

## Frontend Development

The React SPA lives in `web/`. For active development, run the Vite dev server
alongside the Go API server — changes to TypeScript files are applied instantly
via HMR (hot module replacement) without rebuilding Go.

```bash
# Terminal 1 — Go API (port 8080)
make run

# Terminal 2 — Vite dev server with HMR (port 5173)
make ui-dev
# Open http://localhost:5173 — all /api/* calls are proxied to :8080
```

### Build the production bundle

```bash
make ui-build          # compiles web/ → web/dist/
make build             # ui-build + templ generate + go build (single command)
```

The compiled frontend is baked into the Go binary via `//go:embed`. No CDN or
separate static file server is needed in production.

### Admin panel

A server-rendered admin panel using `templ` + `htmx` is available at `/admin/users`
(requires an admin JWT):

```bash
# Get an admin token via login, then open in browser:
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Password1!"}' | jq .access_token
# Then open: http://localhost:8080/admin/users
```

After modifying `.templ` files, regenerate the Go code:

```bash
make templ-generate
# or
make generate
```

### Cache-control strategy

| Asset | `Cache-Control` header | Reason |
|-------|----------------------|--------|
| `/assets/*.js`, `/assets/*.css` | `public, max-age=31536000, immutable` | Vite adds content hashes — safe to cache forever |
| `index.html` | `no-store` | Must always be fresh so the browser gets new asset hashes |
| `/favicon.ico`, images | `public, max-age=86400` | 1-day TTL |
| `/api/*` responses | `no-store` | Authenticated data must never be cached by proxies |

## Documentation

The full developer documentation lives in [`mkdocs/`](./mkdocs/) and is built with [MkDocs + Material](https://squidfunk.github.io/mkdocs-material/). A `docs` service is included in `docker-compose.yml`.

### View the docs

**Option A — Docker (no install required):**

```bash
docker-compose up -d docs
# Then open: http://localhost:8001
```

**Option B — Local Python:**

```bash
cd mkdocs
pip install mkdocs-material        # once
mkdocs serve                       # http://localhost:8000
```

### Update the docs

All documentation source files are Markdown inside [`mkdocs/content/`](./mkdocs/content/):

| File | Content |
|------|---------|
| `index.md` | Home — service map, architecture, request lifecycle |
| `getting-started.md` | Setup guide (Docker + Windows + local dev) |
| `swagger.md` | Swagger UI walkthrough |
| `auth-users.md` | Auth & user API — full curl examples and error reference |
| `jaeger.md` | Distributed tracing guide |
| `prometheus.md` | Metrics catalogue and PromQL cookbook |
| `grafana.md` | Dashboard panels and Grafana tips |

Edit any `.md` file — the browser auto-reloads when running `mkdocs serve` or the Docker docs service.

To add a new page:
1. Create `mkdocs/content/<page>.md`
2. Add it to the `nav:` section in `mkdocs/mkdocs.yml`

### Regenerate the OpenAPI spec

After changing handler annotations, regenerate and commit the spec:

```bash
make swagger          # regenerates docs/swagger.json + swagger.yaml
make swagger-check    # CI gate — fails if spec is stale
```

The Swagger UI at `http://localhost:8080/swagger/index.html` is served directly from the running API and is disabled when `APP_ENV=production`.

## Observability

Sprint 5 adds a full Prometheus + Grafana stack. Start everything with:

```bash
docker-compose up -d
```

| Service | URL | Credentials |
|---------|-----|-------------|
| API | http://localhost:8080 | — |
| Swagger UI | http://localhost:8080/swagger/index.html | — |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3001 | admin / admin |
| Jaeger | http://localhost:16686 | — |
| Docs | http://localhost:8000 | — |
| Raw metrics | http://localhost:8080/metrics | — |

### Distributed tracing

Traces are exported to Jaeger via OTLP gRPC (Sprint 6).

After `docker-compose up`, make any API call and then open the Jaeger UI at `http://localhost:16686`.
Select service **kleido** to see the trace with all child spans (HTTP → handler → service → repository).

### Log–trace correlation

Every log line emitted inside a traced request includes `trace_id` and `span_id` fields
in JSON output. To correlate:

1. Find a log line of interest — copy its `trace_id` value.
2. Open the Jaeger UI → Search → paste the `trace_id` in the **Trace ID** field.

### Disabling tracing

Set `OTEL_ENABLED=false` to run without tracing. All span instrumentation becomes a no-op
and no connections are made to the OTLP endpoint.

The **Kleido — API Overview** dashboard is auto-provisioned in Grafana. It contains:

| Panel | Metric | Description |
|-------|--------|-------------|
| Requests / sec | `kleido_http_requests_total` | Current request throughput |
| Error rate | `kleido_http_requests_total{status_class=~"4xx\|5xx"}` | Fraction of error responses; yellow ≥ 1 %, red ≥ 5 % |
| Inflight requests | `kleido_http_requests_inflight` | Gauge of concurrent requests being served |
| Cache hit rate | `kleido_cache_operations_total` | Fraction of cache GETs that hit; red < 50 %, green ≥ 80 % |
| Request rate by path | `kleido_http_requests_total` | Per-route time-series (chi route pattern, not raw URL) |
| Request latency p50/p95/p99 | `kleido_http_request_duration_seconds` | Latency percentiles per route |
| DB query p99 by operation | `kleido_db_query_duration_seconds` | Database latency; yellow > 50 ms, red > 100 ms |
| Panics (5 m window) | `kleido_http_panics_total` | Count of recovered panics; any value ≥ 1 is red |

### Exported metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kleido_http_requests_total` | Counter | `method`, `path`, `status_class` | Total HTTP requests handled |
| `kleido_http_request_duration_seconds` | Histogram | `method`, `path` | HTTP request latency |
| `kleido_http_requests_inflight` | Gauge | — | Currently active HTTP requests |
| `kleido_http_panics_total` | Counter | — | Panics recovered by the panic middleware |
| `kleido_db_query_duration_seconds` | Histogram | `operation` | Database query latency |
| `kleido_db_errors_total` | Counter | `operation` | Database errors (not-found excluded) |
| `kleido_cache_operations_total` | Counter | `operation`, `result` | Cache operations (`get`/`set`) and outcomes (`hit`/`miss`/`ok`/`error`) |

## Security

See [SECURITY.md](./SECURITY.md) for the full threat model, security architecture, and vulnerability reporting process. `gosec` runs in CI and all HIGH/CRITICAL findings are treated as blocking.

## Testing

The project has four test layers. See [Testing Strategy](mkdocs/content/testing.md) for the full rationale.

| Layer | Command | What it tests |
|-------|---------|---------------|
| Unit | `make test` | Handlers, services, middleware, auth — all with mocked dependencies |
| Integration | `make test-integration` | Postgres and Redis repository implementations against real containers |
| E2E | `make test-e2e` | Full HTTP server (real router + real DB + real Redis) covering every API scenario |
| Coverage gate | `make test-coverage-check` | Fails CI if `internal/service/` < 80 % or `pkg/apperror/` < 90 % |

```bash
# Unit tests (no Docker required)
make test                   # go test -race -count=1 ./...

# Integration tests (requires Docker)
make test-integration       # go test -race -tags=integration ./internal/repository/...

# E2E tests (requires Docker; builds UI first)
make test-e2e               # go test -tags=e2e -timeout=10m ./cmd/api/

# Coverage report
make test-coverage          # → coverage.html
```

Coverage is uploaded to [Codecov](https://codecov.io/gh/nowi5/kleido) on every push to `main`.
To activate: connect the repo at codecov.io and add `CODECOV_TOKEN` to GitHub Secrets.

> **Windows note:** The `-race` flag requires CGO. On Windows, install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or use WSL 2 to run tests with the race detector. Without a C compiler, omit `-race`: `go test -count=1 ./...`

## Contributing

- Fork → branch from `main` → PR
- `make check` must pass before requesting review
- Branch naming: `feat/`, `fix/`, `chore/`, `docs/`
- Commit message format: `type(scope): short description` ([Conventional Commits](https://www.conventionalcommits.org/))

## License

MIT License — Copyright (c) 2024 nowi5
