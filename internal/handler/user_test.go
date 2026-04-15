package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kleido/internal/handler"
	"kleido/internal/middleware"
	"kleido/internal/model"
	"kleido/pkg/apperror"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Mock user service ---

type mockUserSvcH struct {
	user      *model.User
	users     []*model.UserResponse
	total     int64
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func (m *mockUserSvcH) GetByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return m.user, m.getErr
}

func (m *mockUserSvcH) GetByEmail(_ context.Context, _ string) (*model.User, error) {
	return m.user, m.getErr
}

func (m *mockUserSvcH) Create(_ context.Context, email, _, role string) (*model.User, error) {
	return &model.User{ID: uuid.New(), Email: email, Role: role, IsActive: true}, nil
}

func (m *mockUserSvcH) List(_ context.Context, _, _ int) ([]*model.UserResponse, int64, error) {
	return m.users, m.total, m.listErr
}

func (m *mockUserSvcH) Update(_ context.Context, _ uuid.UUID, _ *model.UpdateUserRequest, _ string) (*model.User, error) {
	return m.user, m.updateErr
}

func (m *mockUserSvcH) Delete(_ context.Context, _ uuid.UUID) error {
	return m.deleteErr
}

func (m *mockUserSvcH) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

// --- Helpers ---

// ctxWithRole injects userID and role into a request context, simulating JWT middleware.
func ctxWithRole(r *http.Request, userID, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.CtxKeyRole, role)
	return r.WithContext(ctx)
}

// withChiParam wraps a request in a chi router context so chi.URLParam works.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func defaultUser(id uuid.UUID) *model.User {
	return &model.User{ID: id, Email: "u@example.com", Role: "user", IsActive: true}
}

// --- GetMe tests ---

func TestGetMe_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(id)}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/me", nil)
	req = ctxWithRole(req, id.String(), "user")
	h.GetMe(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetMe_MissingUserID_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	// No context values injected — simulates a request that bypassed JWT middleware.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/me", nil)
	h.GetMe(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestGetMe_InvalidUUID_Returns401(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/me", nil)
	req = ctxWithRole(req, "not-a-uuid", "user")
	h.GetMe(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestGetMe_NotFound_Returns404(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{getErr: apperror.NotFound("user")}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/me", nil)
	req = ctxWithRole(req, uuid.New().String(), "user")
	h.GetMe(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// --- GetUser tests ---

func TestGetUser_OwnProfile_NonAdmin(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(id)}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/"+id.String(), nil)
	req = ctxWithRole(req, id.String(), "user")
	req = withChiParam(req, "id", id.String())
	h.GetUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetUser_OtherUser_NonAdmin_Forbidden(t *testing.T) {
	t.Parallel()

	targetID := uuid.New()
	callerID := uuid.New().String()
	svc := &mockUserSvcH{user: defaultUser(targetID)}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/"+targetID.String(), nil)
	req = ctxWithRole(req, callerID, "user")
	req = withChiParam(req, "id", targetID.String())
	h.GetUser(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rr.Code)
	}
}

func TestGetUser_Admin_AnyUser(t *testing.T) {
	t.Parallel()

	targetID := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(targetID)}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/"+targetID.String(), nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", targetID.String())
	h.GetUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()

	targetID := uuid.New()
	svc := &mockUserSvcH{getErr: apperror.NotFound("user")}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/"+targetID.String(), nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", targetID.String())
	h.GetUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestGetUser_InvalidUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users/not-a-uuid", nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", "not-a-uuid")
	h.GetUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// --- ListUsers tests ---

func TestListUsers_Admin(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{
		users: []*model.UserResponse{{ID: id, Email: "a@b.com", Role: "user"}},
		total: 1,
	}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users", nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	h.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["data"]; !ok {
		t.Error("response must contain 'data' field")
	}
}

func TestListUsers_NonAdmin_Forbidden(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/users", nil)
	req = ctxWithRole(req, uuid.New().String(), "user")
	h.ListUsers(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rr.Code)
	}
}

// --- UpdateUser tests ---

func TestUpdateUser_ValidBody(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(id)}
	h := handler.NewUserHandler(svc)

	body := []byte(`{"email":"new@example.com"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/users/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithRole(req, id.String(), "user")
	req = withChiParam(req, "id", id.String())
	h.UpdateUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateUser_InvalidUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/users/bad-uuid", nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", "bad-uuid")
	h.UpdateUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestUpdateUser_OtherUser_NonAdmin_Forbidden(t *testing.T) {
	t.Parallel()

	targetID := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(targetID)}
	h := handler.NewUserHandler(svc)

	body := []byte(`{"email":"new@example.com"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/users/"+targetID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithRole(req, uuid.New().String(), "user") // different caller
	req = withChiParam(req, "id", targetID.String())
	h.UpdateUser(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rr.Code)
	}
}

func TestUpdateUser_InvalidJSON(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{user: defaultUser(id)}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/users/"+id.String(), bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithRole(req, id.String(), "user")
	req = withChiParam(req, "id", id.String())
	h.UpdateUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// --- DeleteUser tests ---
// Note: admin role is enforced by RequireRole middleware in routes.go, not the handler itself.
// The handler only validates the UUID and calls the service.

func TestDeleteUser_Admin(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/users/"+id.String(), nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", id.String())
	h.DeleteUser(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Errorf("delete response must have no body, got %q", rr.Body.String())
	}
}

func TestDeleteUser_InvalidUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcH{}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/users/not-a-uuid", nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", "not-a-uuid")
	h.DeleteUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcH{deleteErr: apperror.NotFound("user")}
	h := handler.NewUserHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/users/"+id.String(), nil)
	req = ctxWithRole(req, uuid.New().String(), "admin")
	req = withChiParam(req, "id", id.String())
	h.DeleteUser(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}
