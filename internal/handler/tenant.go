package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"kleido/internal/service"
	"kleido/pkg/apperror"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type TenantHandler struct {
	svc service.TenantService
}

func NewTenantHandler(svc service.TenantService) *TenantHandler {
	return &TenantHandler{svc: svc}
}

func (h *TenantHandler) List(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.svc.List(r.Context())
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	responses := make([]TenantResponse, len(tenants))
	for i, t := range tenants {
		responses[i] = TenantResponse{
			ID:   t.ID,
			Name: t.Name,
			Slug: t.Slug,
		}
	}

	respondJSON(w, r, http.StatusOK, responses)
}

func (h *TenantHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		apperror.WriteError(w, apperror.BadRequest("invalid tenant id", err))
		return
	}

	tenant, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperror.WriteError(w, err)
		return
	}

	if tenant == nil {
		apperror.WriteError(w, apperror.NotFound("tenant"))
		return
	}

	respondJSON(w, r, http.StatusOK, tenant.ToResponse())
}

type TenantResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

func respondJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			slog.Default().Error("encode response", "error", err)
		}
	}
}