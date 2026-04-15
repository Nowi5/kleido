# AGENTS.md — Kleido

## Module name
The Go module is `kleido` (not `myapp`). All import paths start with `kleido/`.

## Multi-Tenancy
This project implements tenant isolation by design. Key components:

- **Model**: `internal/model/tenant.go` - Tenant struct with ID, Name, Slug, Settings
- **Repository**: `internal/repository/postgres/tenant.go` - CRUD operations for tenants
- **Service**: `internal/service/tenant.go` - Business logic for tenant management
- **Middleware**: `internal/middleware/tenant.go` - Tenant resolution from subdomain/query params
- **Handler**: `internal/handler/tenant.go` - HTTP endpoints for tenant operations
- **Context**: `internal/reqctx/reqctx.go` - TenantID context utilities

### Tenant Resolution Priority
1. Subdomain: `tenant1.example.com` → resolves to tenant with slug `tenant1`
2. Query Parameter: `?tenant_id=<uuid>` - explicit tenant ID

### Tenant-Aware Endpoints
- `GET /api/v1/tenants` - List all active tenants
- `GET /api/v1/tenants/{id}` - Get tenant by ID
- Any endpoint requiring tenant context uses `middleware.RequireTenantID`

### Database
- `migrations/000002_create_tenants.up.sql` - Creates tenants table with tenant_id FK on users
- Default tenant "default" is seeded on startup via `cmd/api/seed.go`

### Testing
Tenant tests are in `internal/middleware/tenant_test.go` and `internal/handler/tenant_test.go`.

## Build order matters
`make build` runs `ui-build` → `templ-generate` → `go build` in sequence. If you run `go build` directly without first building `web/dist/` and generating `*_templ.go` files, the build will fail.

- `web/embed.go` embeds `web/dist/` via `//go:embed dist` — the directory must exist.
- `web/components/*.templ` generates `*_templ.go` via `templ generate ./web/components/...`. These files must exist before `go build`.

## Code generation tools
- **Mockery v2** (`make generate` / `go generate ./...`): generates into `internal/mocks/{repository,service,middleware}` with packages `mockrepo`, `mocksvc`, `mockmw`. Config in `.mockery.yaml`.
- **swag**: `swag init -g cmd/api/main.go --output docs/` + `swag fmt`. Commit the generated `docs/swagger.json` and `docs/swagger.yaml`.
- **templ**: `templ generate ./web/components/...`. Run after any `.templ` file change.

## Test layers
| Command | Tags | Scope |
|---------|------|-------|
| `make test` | none | Unit tests only (mocked deps); excludes integration packages |
| `make test-integration` | `integration` | `internal/repository/...` against real Postgres + Redis |
| `make test-e2e` | `e2e` | Full server with testcontainers; requires `web/dist/` + `*_templ.go` |

`-race` is used in all test commands (requires CGO / GCC). Omit `-race` on Windows if TDM-GCC is unavailable.

## Lint & quality gates
- `make lint` uses golangci-lint with config in `.golangci.yml`.
- **golangci-lint version**: Defined in `.github/workflows/ci.yml` (`GOLANGCI_LINT_VERSION` env var) and `.github/actions/go-setup/action.yml` (`golangci-lint-version` input).
- `wrapcheck` is configured but ignores `cmd/` — do not add wrapper errors in entry points.
- Coverage gates (enforced in CI): `internal/service/` ≥ 80%, `pkg/apperror/` ≥ 90%.

## Error handling
- Only `internal/config/` calls `os.Getenv`. No `os.Getenv` elsewhere.
- All errors go through `pkg/apperror/AppError` — never use `http.Error()` outside handlers.

## Docker / CI quirks
- The `go-setup` GitHub Action (`./.github/actions/go-setup/`) is critical: it builds the UI, installs templ, and generates `*_templ.go` before any Go compilation. Every job that compiles or tests Go code uses this action.
- Docker build is multi-stage: `node:20-alpine` (UI) → `golang:1.25-alpine` (Go, templ gen) → `alpine:3.20` (runtime).
- The Docker image runs migrations on startup inside `cmd/api/main.go`.

## Migration tool
Uses `golang-migrate`. Create with `make migrate-create NAME=description` (generates `000001_*` sequential files in `migrations/`). Run with `make migrate-up` / `migrate-down`.

## Required tools (via `tools.go`)
`templ`, `golangci-lint`, `swag`, `mockery/v2`, `govulncheck`, `gotestsum`.

## Windows notes
- On Windows, run `go run ./cmd/api/` instead of `make run`.
- Omit `-race` flag for tests if CGO/GCC is not available.
- Use Docker Desktop for Windows with WSL 2 backend for `docker-compose` workflows.
