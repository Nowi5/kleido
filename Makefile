.PHONY: build run test test-integration test-e2e test-coverage test-ci test-coverage-check \
        lint lint-fix fmt vet check \
        swagger swagger-check generate templ-generate \
        migrate-up migrate-down migrate-create \
        gen-keys ui-build ui-install ui-dev \
        docker-build docker-up docker-down stack-smoke stack-down \
        vuln-check scan clean \
        hooks-install

APP         := kleido
BUILD_DIR   := ./bin
CMD_API     := ./cmd/api
CMD_CLI     := ./cmd/cli
MIGRATE_URL ?= $(shell grep ^DATABASE_URL .env 2>/dev/null | cut -d= -f2-)
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -ldflags="-s -w -X main.version=$(VERSION)"

# TEST_PKGS: packages in scope for unit tests.
# Excludes packages that are either:
#   - auto-generated (docs, web/components, *_templ.go)
#   - integration-only and require Docker (internal/repository/postgres, redis)
#   - third-party code pulled in by npm (web/node_modules)
TEST_PKGS := $(shell go list ./... | grep -Ev \
	'/docs$$|/web/node_modules|/web/components$$|/internal/repository/postgres$$|/internal/repository/redis$$')

# ── Build ─────────────────────────────────────────────────────────────────────
# build runs ui-build and templ-generate first so web/dist/ and *_templ.go files
# are present before go build embeds them. No manual prerequisite steps needed.
build: ui-build templ-generate
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP)-api  $(CMD_API)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP)-cli  $(CMD_CLI)

run:
	go run $(CMD_API)/

# ── Test ──────────────────────────────────────────────────────────────────────
test:
	go test -race -count=1 $(TEST_PKGS)

test-integration:
	go test -race -tags=integration -count=1 ./internal/repository/...

# test-e2e: end-to-end tests against a real HTTP server backed by testcontainers.
# Requires Docker. Requires web/dist to be built (run make ui-build first).
# These tests are intentionally excluded from the default `make test` target
# because they take ~60 s per function due to container startup.
test-e2e: ui-build templ-generate
	go test -v -race -tags=e2e -count=1 -timeout=10m ./cmd/api/

test-coverage:
	go test -coverprofile=coverage.out $(TEST_PKGS)
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1
	@echo "Open: coverage.html"

# test-ci: run tests with gotestsum and emit JUnit XML + coverage profile for CI/Sonar.
# Requires: go install gotest.tools/gotestsum@latest
# Produces: reports/unit.xml (test execution) and coverage.out (coverage profile)
test-ci:
	@mkdir -p reports
	gotestsum --junitfile reports/unit.xml --format pkgname \
		-- -race -count=1 -coverprofile=coverage.out -covermode=atomic $(TEST_PKGS)

# test-coverage-check: enforce per-package coverage gates.
# Fails the build if internal/service/ < 80% or pkg/apperror/ < 90%.
test-coverage-check:
	go test -coverprofile=coverage.out $(TEST_PKGS)
	@go tool cover -func=coverage.out | awk ' \
		/^kleido\/internal\/service\// { \
			pct = $$NF+0; if (pct < 80) { \
				printf "FAIL internal/service coverage %.1f%% < 80%%\n", pct; exit 1 } } \
		/^kleido\/pkg\/apperror\// { \
			pct = $$NF+0; if (pct < 90) { \
				printf "FAIL pkg/apperror coverage %.1f%% < 90%%\n", pct; exit 1 } }' \
		&& echo "✓ Coverage gates passed"

# ── Quality ───────────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet lint test
	@echo "✓ All checks passed"

# ── Documentation ─────────────────────────────────────────────────────────────
swagger:
	swag init -g cmd/api/main.go --output docs/
	swag fmt

swagger-check:
	@swag init -g cmd/api/main.go --output /tmp/swagger_check/ --packageName docs 2>/dev/null
	@diff -q docs/swagger.json /tmp/swagger_check/swagger.json \
		&& diff -q docs/swagger.yaml /tmp/swagger_check/swagger.yaml \
		|| (echo "ERROR: Swagger docs are stale. Run: make swagger" && exit 1)
	@echo "✓ Swagger docs are up to date"

# ── Code generation ───────────────────────────────────────────────────────────
generate: templ-generate
	go generate ./...

# templ-generate compiles .templ files to *_templ.go.
# Requires: go install github.com/a-h/templ/cmd/templ@latest
templ-generate:
	templ generate ./web/components/...

# ── Database ──────────────────────────────────────────────────────────────────
migrate-up:
	migrate -path ./migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(MIGRATE_URL)" down 1

migrate-create:
	@test -n "$(NAME)" || (echo "Usage: make migrate-create NAME=description" && exit 1)
	migrate create -ext sql -dir ./migrations -seq $(NAME)

# ── Keys ──────────────────────────────────────────────────────────────────────
gen-keys:
	bash scripts/gen-keys.sh

# ── UI ────────────────────────────────────────────────────────────────────────
ui-build:
	cd web && npm ci && npm run build

ui-install:
	cd web && npm ci

ui-dev:
	cd web && npm run dev

# ── Security ──────────────────────────────────────────────────────────────────
# vuln-check: scan Go dependencies for known CVEs using govulncheck.
# Requires: go install golang.org/x/vuln/cmd/govulncheck@latest
vuln-check:
	govulncheck ./...

# scan: full security sweep — Go CVEs + container image CVE scan.
# Requires: govulncheck, docker, trivy (https://aquasecurity.github.io/trivy/)
scan: vuln-check docker-build
	trivy image --exit-code 1 --severity HIGH,CRITICAL --ignore-unfixed \
		--ignorefile .trivyignore \
		$(APP):latest

# ── Docker ────────────────────────────────────────────────────────────────────
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(APP):$(VERSION) -t $(APP):latest .

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down

# stack-smoke: bring up the full stack and verify the API health endpoint.
stack-smoke:
	docker-compose up --build -d
	@echo "Waiting for API to become healthy…"
	@for i in $$(seq 1 30); do \
		docker-compose exec -T api wget -qO- http://localhost:8080/healthz && echo " ✓ healthy" && exit 0; \
		sleep 2; \
	done; echo "FAIL: API did not become healthy in 60s" && exit 1

stack-down:
	docker-compose down -v

# ── Git hooks ─────────────────────────────────────────────────────────────────
# hooks-install: configure git to use the .githooks/ directory.
# Run once after cloning: make hooks-install
hooks-install:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo "✓ Git hooks installed from .githooks/"

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html web/dist docs/
	find . -name "*.test" -delete
