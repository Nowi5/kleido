// Package handler implements the HTTP handlers for the kleido API.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type healthResponse struct {
	Status string `json:"status"`
}

// ReadyzResponse is the payload returned by the readiness probe.
type ReadyzResponse struct {
	Status string `json:"status" example:"ready"`
	DB     bool   `json:"db"     example:"true"`
	Redis  bool   `json:"redis"  example:"true"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("encode response", slog.Any("error", err))
	}
}

// Healthz godoc
//
//	@Summary		Liveness probe
//	@Description	Returns 200 if the process is running. No downstream checks.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	healthResponse	"status: ok"
//	@Router			/healthz [get]
func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}

// Readyz godoc
//
//	@Summary		Readiness probe
//	@Description	Returns 200 when PostgreSQL and Redis are reachable. Returns 503 if either is down.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	ReadyzResponse
//	@Failure		503	{object}	ReadyzResponse
//	@Router			/readyz [get]
func Readyz(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		dbOK := pool.Ping(ctx) == nil
		redisOK := rdb.Ping(ctx).Err() == nil

		resp := ReadyzResponse{
			DB:    dbOK,
			Redis: redisOK,
		}

		if dbOK && redisOK {
			resp.Status = "ready"
			writeJSON(w, http.StatusOK, resp)
		} else {
			resp.Status = "degraded"
			writeJSON(w, http.StatusServiceUnavailable, resp)
		}
	}
}
