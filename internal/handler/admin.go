package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"kleido/internal/logger"
	"kleido/internal/middleware"
	"kleido/internal/service"
	"kleido/pkg/apperror"
	"kleido/web/components"
)

// AdminHandler handles server-rendered admin panel requests using templ + htmx.
type AdminHandler struct {
	userSvc service.UserService
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(userSvc service.UserService) *AdminHandler {
	return &AdminHandler{userSvc: userSvc}
}

// UserList renders the admin user list page.
// When the HX-Request header is present, it returns only the <tbody> fragment
// so htmx can swap it inline without a full page reload.
func (h *AdminHandler) UserList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	users, total, err := h.userSvc.List(ctx, 100, 0)
	if err != nil {
		log.ErrorContext(ctx, "admin user list failed", slog.Any("error", err))
		apperror.WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Header.Get("HX-Request") == "true" {
		// htmx partial — return only the updated <tbody> rows.
		_ = components.AdminUsersTableBody(users).Render(ctx, w)
		return
	}

	// Full-page render — include the Bearer token so htmx can attach it to
	// subsequent DELETE requests made from the admin table.
	token := extractBearerToken(r)
	_ = components.AdminUsers(users, total, token).Render(ctx, w)
}

// UserDelete handles the htmx DELETE request from the admin table.
// Returns 200 with an empty body — htmx interprets this as success and removes
// the target row (hx-swap="outerHTML" on the button).
func (h *AdminHandler) UserDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		apperror.WriteError(w, apperror.BadRequest("invalid user ID", err))
		return
	}

	// callerRole is set by JWT middleware; always "admin" here because the route
	// group uses RequireRole("admin"). Captured for audit logging.
	callerRole, _ := ctx.Value(middleware.CtxKeyRole).(string)

	if err := h.userSvc.Delete(ctx, id); err != nil {
		log.ErrorContext(ctx, "admin delete failed",
			slog.String("user_id", id.String()),
			slog.Any("error", err),
		)
		apperror.WriteError(w, err)
		return
	}

	log.InfoContext(ctx, "user deleted by admin",
		slog.String("user_id", id.String()),
		slog.String("caller_role", callerRole),
	)

	// Empty 200 body — htmx removes the target row.
	w.WriteHeader(http.StatusOK)
}

// extractBearerToken pulls the raw token value from an "Authorization: Bearer <token>"
// header. Returns an empty string if the header is absent or malformed.
func extractBearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	after, ok := strings.CutPrefix(v, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(after)
}
