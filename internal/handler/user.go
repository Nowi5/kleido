package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"kleido/internal/logger"
	"kleido/internal/middleware"
	"kleido/internal/model"
	"kleido/internal/service"
	"kleido/pkg/apperror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var userTracer = otel.Tracer("kleido/handler")

// ListUsersResponse is the paginated envelope for the ListUsers endpoint.
type ListUsersResponse struct {
	Data    []*model.UserResponse `json:"data"`
	Total   int64                 `json:"total"`
	Page    int                   `json:"page"`
	PerPage int                   `json:"per_page"`
}

// UserHandler handles user-related HTTP requests.
type UserHandler struct {
	svc service.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// parseIntQuery reads a query parameter as int, falling back to defaultVal.
func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// GetMe godoc
//
//	@Summary		Get the authenticated user's profile
//	@Description	Returns the profile of the currently authenticated user.
//	@Tags			users
//	@Produce		json
//	@Success		200	{object}	model.UserResponse
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/users/me [get]
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx, span := userTracer.Start(r.Context(), "handler/GetMe")
	defer span.End()

	userIDStr, ok := ctx.Value(middleware.CtxKeyUserID).(string)
	if !ok || userIDStr == "" {
		err := apperror.Unauthorized("missing user ID in token")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		appErr := apperror.Unauthorized("invalid user ID in token")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	span.SetAttributes(attribute.String("user.id", userID.String()))

	user, err := h.svc.GetByID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, user.ToResponse())
}

// GetUser godoc
//
//	@Summary		Get user by ID
//	@Description	Returns a single active user by UUID. Admins can see all users; regular users can only fetch their own profile.
//	@Tags			users
//	@Produce		json
//	@Param			id	path		string	true	"User UUID"	Format(uuid)
//	@Success		200	{object}	model.UserResponse
//	@Failure		400	{object}	apperror.ErrorResponse	"Invalid UUID"
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		403	{object}	apperror.ErrorResponse	"Non-admin fetching another user"
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/users/{id} [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := userTracer.Start(r.Context(), "handler/GetUser")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		appErr := apperror.BadRequest("invalid user ID", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	span.SetAttributes(attribute.String("user.id", targetID.String()))

	callerID, ok := ctx.Value(middleware.CtxKeyUserID).(string)
	if !ok {
		callerID = ""
	}
	callerRole, ok := ctx.Value(middleware.CtxKeyRole).(string)
	if !ok {
		callerRole = ""
	}

	if callerRole != "admin" && callerID != targetID.String() {
		appErr := apperror.Forbidden("cannot fetch another user's profile")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	user, err := h.svc.GetByID(ctx, targetID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, user.ToResponse())
}

// ListUsers godoc
//
//	@Summary		List users
//	@Description	Returns a paginated list of all users. Admin only.
//	@Tags			users
//	@Produce		json
//	@Param			page		query		int	false	"Page number (1-based)"		default(1)
//	@Param			per_page	query		int	false	"Items per page (max 100)"	default(20)
//	@Success		200			{object}	ListUsersResponse
//	@Failure		401			{object}	apperror.ErrorResponse
//	@Failure		403			{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/users [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx, span := userTracer.Start(r.Context(), "handler/ListUsers")
	defer span.End()

	callerRole, ok := ctx.Value(middleware.CtxKeyRole).(string)
	if !ok {
		callerRole = ""
	}
	if callerRole != "admin" {
		appErr := apperror.Forbidden("admin access required")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	page := parseIntQuery(r, "page", 1)
	perPage := parseIntQuery(r, "per_page", 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 1
	}
	if perPage > 100 {
		perPage = 100
	}

	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.per_page", perPage),
	)

	offset := (page - 1) * perPage
	users, total, err := h.svc.List(ctx, perPage, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, ListUsersResponse{
		Data:    users,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// UpdateUser godoc
//
//	@Summary		Update user
//	@Description	Partially updates a user's email, role, or active status. Role changes require admin privileges; non-admins may only update their own profile.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"User UUID"
//	@Param			body	body		model.UpdateUserRequest	true	"Fields to update (all optional)"
//	@Success		200		{object}	model.UserResponse
//	@Failure		400		{object}	apperror.ErrorResponse
//	@Failure		401		{object}	apperror.ErrorResponse
//	@Failure		403		{object}	apperror.ErrorResponse	"Non-admin updating another user"
//	@Failure		404		{object}	apperror.ErrorResponse
//	@Failure		409		{object}	apperror.ErrorResponse	"Email already taken"
//	@Security		BearerAuth
//	@Router			/users/{id} [put]
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := userTracer.Start(r.Context(), "handler/UpdateUser")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		appErr := apperror.BadRequest("invalid user ID", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	span.SetAttributes(attribute.String("user.id", targetID.String()))

	callerID, ok := ctx.Value(middleware.CtxKeyUserID).(string)
	if !ok {
		callerID = ""
	}
	callerRole, ok := ctx.Value(middleware.CtxKeyRole).(string)
	if !ok {
		callerRole = ""
	}

	if callerRole != "admin" && callerID != targetID.String() {
		appErr := apperror.Forbidden("cannot update another user's profile")
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	var req model.UpdateUserRequest
	if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil {
		appErr := apperror.BadRequest("invalid request body", decErr)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	user, err := h.svc.Update(ctx, targetID, &req, callerRole)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	span.SetStatus(codes.Ok, "")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, r, user.ToResponse())
}

// DeleteUser godoc
//
//	@Summary		Delete user
//	@Description	Soft-deletes a user (sets is_active = false). Admin only.
//	@Tags			users
//	@Produce		json
//	@Param			id	path	string	true	"User UUID"
//	@Success		204	"No content"
//	@Failure		401	{object}	apperror.ErrorResponse
//	@Failure		403	{object}	apperror.ErrorResponse
//	@Failure		404	{object}	apperror.ErrorResponse
//	@Security		BearerAuth
//	@Router			/users/{id} [delete]
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := userTracer.Start(r.Context(), "handler/DeleteUser")
	defer span.End()

	idStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		appErr := apperror.BadRequest("invalid user ID", err)
		span.RecordError(appErr)
		span.SetStatus(codes.Error, appErr.Error())
		apperror.WriteError(w, appErr)
		return
	}

	span.SetAttributes(attribute.String("user.id", targetID.String()))

	if err := h.svc.Delete(ctx, targetID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		apperror.WriteError(w, err)
		return
	}

	_ = logger.FromContext(ctx) // ensure context logger is available for future use
	span.SetStatus(codes.Ok, "")
	w.WriteHeader(http.StatusNoContent)
}
