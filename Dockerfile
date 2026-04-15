# ── Stage 1: UI build ─────────────────────────────────────────────────────────
FROM node:20-alpine AS ui-builder

WORKDIR /app

# Cache npm install separately from source changes.
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci

COPY web/ ./web/
RUN cd web && npm run build

# ── Stage 2: Go build ─────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder

ARG VERSION=dev

# git: needed by go mod download for VCS stamping.
# ca-certificates: needed for TLS module downloads.
RUN apk --no-cache add git ca-certificates

WORKDIR /app

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy source.
COPY . .

# Overlay the pre-built UI assets from Stage 1.
COPY --from=ui-builder /app/web/dist ./web/dist

# Install templ and generate *_templ.go files.
RUN go install github.com/a-h/templ/cmd/templ@v0.2.778
RUN templ generate ./web/components/...

# Build a statically linked binary with debug info stripped.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -extldflags '-static'" \
    -o /bin/kleido-api \
    ./cmd/api

# ── Stage 3: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: needed for TLS outbound connections.
# tzdata: needed for time.LoadLocation in any timezone logic.
# wget: needed for the container healthcheck.
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

COPY --from=go-builder /bin/kleido-api /bin/kleido-api
COPY --from=go-builder /app/migrations /app/migrations

# Run as a non-root user for security.
RUN addgroup -S kleido && adduser -S kleido -G kleido -u 1001
USER 1001

EXPOSE 8080

ENTRYPOINT ["/bin/kleido-api"]
