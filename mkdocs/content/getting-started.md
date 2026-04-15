# Getting Started

This page walks you through starting the full Kleido stack from scratch and verifying every service is healthy.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Docker Desktop | 24.x+ | WSL 2 backend recommended on Windows |
| Docker Compose | v2 (`docker compose`) or v1 (`docker-compose`) | Bundled with Docker Desktop |
| OpenSSL | any | Ships with Git for Windows; built-in on macOS/Linux |
| Git | any | To clone the repo |

Go is **not required** — the API is compiled inside the Docker build stage.

---

## Repository layout

```
kleido/                          ← repo root — run docker-compose from here
├── docker-compose.yml           ← full stack (API + services + docs on :8001)
├── mkdocs/                      ← documentation source (this site)
└── cmd/ internal/ migrations/   ← Go application source
```

A single `docker-compose.yml` at the repo root starts the full stack — API, Postgres, Redis, Jaeger, Prometheus, Grafana, and the Docs site.

---

## 1 · Clone

```bash
git clone https://github.com/nowi5/kleido.git
cd kleido                        # stay at the repo root
```

---

## 2 · Copy environment config

=== "Linux / macOS / Git Bash"

    ```bash
    cp .env.example .env
    ```

=== "Windows Command Prompt"

    ```cmd
    copy .env.example .env
    ```

=== "Windows PowerShell"

    ```powershell
    Copy-Item .env.example .env
    ```

The defaults are already wired to the Docker Compose service names — no edits needed for local development.

---

## 3 · Generate RSA keys

JWT tokens are signed with an RSA-4096 private key. The key pair must exist before the API container starts.

=== "Linux / macOS / Git Bash"

    ```bash
    bash scripts/gen-keys.sh
    ```

=== "Windows (OpenSSL directly)"

    ```cmd
    mkdir keys
    openssl genrsa -out keys/private.pem 4096
    openssl rsa -in keys/private.pem -pubout -out keys/public.pem
    ```

    !!! tip
        `openssl` is included with **Git for Windows** and is built into **Windows 10/11** (since 2018).
        Open **Git Bash** or a terminal where `openssl --version` succeeds.

This creates `keys/private.pem` and `keys/public.pem`, bind-mounted read-only into the API container.

---

## 4 · Start the full stack

Run from the **repo root** (`kleido/`):

```bash
docker-compose up --build -d
```

Docker will:

1. Pull base images (first run only)
2. Build the Go binary inside the `golang:1.25-alpine` builder stage
3. Package it into a minimal `alpine:3.20` runtime image
4. Start all 7 services in dependency order (including the docs site on :8000)

!!! note "Windows — Docker context"
    If you see `unable to get image … dockerDesktopLinuxEngine`, run:
    ```bash
    docker context use default
    ```
    Then retry `docker-compose up --build -d`.

!!! tip "Backing services only (local Go development)"
    To run the API from source with only the backing services in Docker:
    ```bash
    docker-compose up -d postgres redis jaeger
    go run ./cmd/api/
    ```

---

## 5 · Verify all services

Run each check below. All should succeed within ~30 seconds of the stack starting.

### API liveness

```bash
curl http://localhost:8080/healthz
```
```json
{"status":"ok"}
```

### API readiness (checks DB + Redis)

```bash
curl http://localhost:8080/readyz
```
```json
{"status":"ready","db":true,"redis":true}
```

### Docs site

Open [http://localhost:8001](http://localhost:8001) — you are reading it right now.

### Prometheus scraping the API

Open [http://localhost:9090/targets](http://localhost:9090/targets).  
The `kleido-api` target should show **State: UP**.

### Grafana dashboard loaded

Open [http://localhost:3001](http://localhost:3001) → log in with `admin` / `admin`.  
The **Kleido — API Overview** dashboard should be visible under **Dashboards**.

### Jaeger receiving traces

Open [http://localhost:16686](http://localhost:16686).  
Make one API call then select **Service: kleido** and click **Find Traces**.

---

## Container status at a glance

Run from the repo root:

```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

Expected output:

```
NAMES                  STATUS
kleido-docs-1          Up 1 minute
kleido-api-1           Up 1 minute (healthy)
kleido-grafana-1       Up 1 minute
kleido-prometheus-1    Up 1 minute
kleido-postgres-1      Up 1 minute (healthy)
kleido-redis-1         Up 1 minute (healthy)
kleido-jaeger-1        Up 1 minute
```

---

## Common operations

All commands run from the **repo root** (`kleido/`) unless noted.

| Task | Command |
|------|---------|
| Stop all containers | `docker-compose down` |
| Rebuild only the API image | `docker-compose up --build -d api` |
| View API logs (live) | `docker logs -f kleido-api-1` |
| View all logs | `docker-compose logs -f` |
| Destroy everything including volumes | `docker-compose down -v` |
| Regenerate RSA keys | `bash scripts/gen-keys.sh` → `docker-compose restart api` |
| Build static docs site | `docker-compose run --rm docs build` → output in `mkdocs/site/` |

---

## Ports reference

| Port | Service | URL |
|------|---------|-----|
| **8001** | **Docs** | **http://localhost:8001** |
| 8080 | API | http://localhost:8080 |
| 8080 | Swagger UI | http://localhost:8080/swagger/index.html |
| 5432 | PostgreSQL | internal — `docker exec -it kleido-postgres-1 psql -U user -d kleido` |
| 6379 | Redis | internal — `docker exec -it kleido-redis-1 redis-cli` |
| 4317 | Jaeger OTLP gRPC | internal — API sends traces here |
| 16686 | Jaeger UI | http://localhost:16686 |
| 9090 | Prometheus | http://localhost:9090 |
| 3001 | Grafana | http://localhost:3001 (admin / admin) |
