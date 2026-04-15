package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kleido/internal/handler"
	"kleido/internal/middleware"
	"kleido/internal/model"
	"kleido/pkg/apperror"

	"github.com/google/uuid"
)

// --- Mock ---

// mockUserSvcAdmin is a test double for service.UserService used in admin tests.
type mockUserSvcAdmin struct {
	users     []*model.UserResponse
	total     int64
	listErr   error
	deleteErr error
}

func (m *mockUserSvcAdmin) GetByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return nil, apperror.NotFound("user")
}
func (m *mockUserSvcAdmin) GetByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, apperror.NotFound("user")
}
func (m *mockUserSvcAdmin) Create(_ context.Context, _, _, _ string) (*model.User, error) {
	return nil, nil //nolint:nilnil
}
func (m *mockUserSvcAdmin) List(_ context.Context, _, _ int) ([]*model.UserResponse, int64, error) {
	return m.users, m.total, m.listErr
}
func (m *mockUserSvcAdmin) Update(_ context.Context, _ uuid.UUID, _ *model.UpdateUserRequest, _ string) (*model.User, error) {
	return nil, nil //nolint:nilnil
}
func (m *mockUserSvcAdmin) Delete(_ context.Context, _ uuid.UUID) error {
	return m.deleteErr
}

func (m *mockUserSvcAdmin) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

// --- Helper ---

// ctxWithAdminRole injects userID and admin role into the request context.
func ctxWithAdminRole(r *http.Request, userID string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.CtxKeyRole, "admin")
	return r.WithContext(ctx)
}

// --- UserList tests ---

func TestAdminHandler_UserList_FullPage(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcAdmin{
		users: []*model.UserResponse{{ID: id, Email: "admin-test@example.com", Role: "user", IsActive: true}},
		total: 1,
	}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	h.UserList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type must be text/html, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "admin-test@example.com") {
		t.Error("full-page response must contain the user's email")
	}
	if !strings.Contains(rr.Body.String(), "<html") {
		t.Error("full-page response must contain <html> tag")
	}
}

func TestAdminHandler_UserList_HtmxPartial(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcAdmin{users: []*model.UserResponse{}, total: 0}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	req.Header.Set("HX-Request", "true")
	h.UserList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("htmx partial must NOT contain <html>")
	}
	if strings.Contains(body, "<head") {
		t.Error("htmx partial must NOT contain <head>")
	}
}

func TestAdminHandler_UserList_ServiceError(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcAdmin{listErr: apperror.Internal(errors.New("db error"))}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	h.UserList(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}

func TestAdminHandler_UserList_BearerTokenInMeta(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcAdmin{users: []*model.UserResponse{}, total: 0}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	req.Header.Set("Authorization", "Bearer test.jwt.token")
	h.UserList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `name="api-token"`) {
		t.Error("full-page response must contain api-token meta tag")
	}
	if !strings.Contains(body, "test.jwt.token") {
		t.Error("full-page response must embed the Bearer token value in the meta tag")
	}
}

// --- UserDelete tests ---

func TestAdminHandler_UserDelete_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcAdmin{}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/admin/users/"+id.String(), nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	req = withChiParam(req, "id", id.String())
	h.UserDelete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.Len() != 0 {
		t.Errorf("delete response must have empty body, got %q", rr.Body.String())
	}
}

func TestAdminHandler_UserDelete_NotFound(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	svc := &mockUserSvcAdmin{deleteErr: apperror.NotFound("user")}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/admin/users/"+id.String(), nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	req = withChiParam(req, "id", id.String())
	h.UserDelete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestAdminHandler_UserDelete_InvalidUUID(t *testing.T) {
	t.Parallel()

	svc := &mockUserSvcAdmin{}
	h := handler.NewAdminHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/admin/users/not-a-uuid", nil)
	req = ctxWithAdminRole(req, uuid.New().String())
	req = withChiParam(req, "id", "not-a-uuid")
	h.UserDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}
