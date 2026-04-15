# Testing Strategy

Kleido uses a **four-layer test pyramid**. Each layer tests at a different level of abstraction and trades off speed against fidelity.

```
          ┌─────────────────┐
          │   E2E tests      │  ← real HTTP server, real DB & Redis (testcontainers)
          │   (slow, high    │    cmd/api/e2e_test.go   tag: e2e
          │    fidelity)     │
          └────────┬────────┘
                   │
          ┌────────▼────────┐
          │  Integration    │  ← real Postgres & Redis (testcontainers)
          │  tests          │    internal/repository/**/*_integration_test.go
          └────────┬────────┘    tag: integration
                   │
          ┌────────▼────────┐
          │  Unit tests     │  ← all dependencies mocked via interfaces
          │  (fast, most    │    internal/**/*_test.go, cmd/api/ui_test.go
          │   numerous)     │
          └─────────────────┘
```

---

## Layer 1 — Unit Tests

**Location:** Every `*_test.go` file that does **not** have a build tag.

**What they test:**
- HTTP handlers (`internal/handler/`) — request decoding, response encoding, status codes, access-control decisions. Use `net/http/httptest` with mock services.
- Service logic (`internal/service/`) — business rules such as bcrypt hashing, JWT generation, lockout counting, token consumption order. Use mock repositories.
- JWT auth (`internal/auth/`) — RS256 sign/parse, expiry, JTI round-trips.
- Middleware (`internal/middleware/`) — JWT validation, rate limiting, security headers, panic recovery.
- Logger, configstore, CLI commands, UI routes, telemetry.

**Run:**
```bash
make test
# or
go test -race -count=1 ./...
```

**Coverage gates enforced in CI:**

| Package | Minimum |
|---------|---------|
| `internal/service/` | 80 % |
| `pkg/apperror/` | 90 % |

---

## Layer 2 — Integration Tests

**Location:** Files tagged `//go:build integration` in `internal/repository/`.

**What they test:**
- `internal/repository/postgres/user_integration_test.go` — all `UserRepository` methods against a real `postgres:16-alpine` container: create, find by ID, find by email, update, delete, list, and unique-constraint violations (409).
- `internal/repository/redis/session_integration_test.go` — all `SessionRepository` methods against a real `redis:7-alpine` container: refresh-token store/validate/revoke, JTI blocklist, sliding-window rate limit, lockout increment/clear, password-reset token store/consume.

Both tests use [testcontainers-go](https://golang.testcontainers.org/) — no external Docker Compose setup required.

**Run:**
```bash
make test-integration
# or
go test -race -tags=integration -count=1 ./internal/repository/...
```

**CI:** Runs in the `integration-test` job with Postgres 16 + Redis 7 as GitHub Actions `services`.

---

## Layer 3 — E2E Tests

**Location:** `cmd/api/e2e_test.go` (build tag `//go:build e2e`)

**What they test:** The full HTTP server — real chi router, real service layer, real Postgres, real Redis — exactly as it runs in production. They automate every scenario exercised during the manual testing session that confirmed the Sprint 10 implementation.

| Test function | Scenarios covered |
|---------------|-------------------|
| `TestE2E_Register` | 201 on success · 409 on duplicate email · 400 on empty fields |
| `TestE2E_Login` | 200 + JWT + httpOnly cookie · 401 wrong password · 400 empty fields |
| `TestE2E_TokenLifecycle` | GET /users/me · token refresh + cookie rotation · logout 204 · revoked token 401 |
| `TestE2E_AccessControl` | 403 non-admin on /users · 401 malformed JWT |
| `TestE2E_PasswordReset` | Forgot (known + unknown → both 200) · reset 200 · single-use 400 · new pass 200 · old pass 401 |
| `TestE2E_BruteForce` | 10 failures → 429 lockout · correct password while locked → 429 |

Each test function spins up its own testcontainers (Postgres + Redis), wires the complete service stack, and starts an `httptest.NewServer`. All cleanup is registered via `t.Cleanup`.

**Run:**
```bash
make test-e2e
# or (manual)
make ui-build && make templ-generate
go test -v -race -tags=e2e -count=1 -timeout=10m ./cmd/api/
```

!!! note "Docker required"
    E2E tests use testcontainers and require Docker Desktop (or a compatible Docker daemon) to be running.

!!! note "UI build required"
    The test binary embeds `web/dist/`. Run `make ui-build` once before running E2E tests, or use `make test-e2e` which does this automatically.

**CI:** Runs in the `e2e-test` job on every push. Blocks the `docker` push job.

---

## Layer 4 — Coverage Gate

A dedicated CI step runs `make test-coverage-check` after the unit tests. It fails the build if any package falls below its threshold.

```bash
make test-coverage-check
```

The gate is implemented in the Makefile using `go tool cover -func`:

```makefile
test-coverage-check:
    go test -coverprofile=coverage.out ./...
    @go tool cover -func=coverage.out | awk ' \
        /^github.com\/nowi5\/kleido\/internal\/service\// { \
            pct = $$NF+0; if (pct < 80) { \
                printf "FAIL internal/service coverage %.1f%% < 80%%\n", pct; exit 1 } } \
        /^github.com\/nowi5\/kleido\/pkg\/apperror\// { \
            pct = $$NF+0; if (pct < 90) { \
                printf "FAIL pkg/apperror coverage %.1f%% < 90%%\n", pct; exit 1 } }' \
    && echo "✓ Coverage gates passed"
```

---

## Coverage Reporting

Coverage is uploaded to [Codecov](https://codecov.io/gh/nowi5/kleido) on every push to `main` via `codecov/codecov-action@v4`.

To activate for your fork:
1. Sign up at [codecov.io](https://codecov.io) and connect your GitHub repository.
2. Copy your repository token.
3. Add it as `CODECOV_TOKEN` in your GitHub repository secrets (`Settings → Secrets → Actions`).

A coverage badge is shown in the [README](../../README.md).

---

## Running Everything Locally

```bash
# Fast feedback loop (no Docker)
make test

# Full confidence (requires Docker)
make test && make test-integration && make test-e2e

# Identical to CI
make test-ci && make test-coverage-check && make test-integration && make test-e2e
```

---

## Adding New Tests

### Unit test
Place it alongside the code it tests. Use the existing mock patterns:
- Handler tests: `net/http/httptest` + mock service structs in `_test.go`
- Service tests: mock repository structs satisfying the `repository.UserRepository` / `repository.SessionRepository` interfaces

### Integration test
Add to `internal/repository/postgres/` or `internal/repository/redis/` with the `//go:build integration` tag. Reuse the `testDB(t)` / `testRedis(t)` helpers.

### E2E test
Add a new top-level `TestE2E_*` function in `cmd/api/e2e_test.go`. Call `newE2EEnv(t)` to get a fresh server, database, and Redis instance.

### Coverage threshold change
Edit the `awk` script inside `test-coverage-check` in the Makefile.
