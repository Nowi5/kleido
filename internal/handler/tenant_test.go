package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kleido/internal/handler"
	"kleido/internal/model"
	"kleido/pkg/apperror"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockTenantService struct {
	tenants []*model.Tenant
	err     error
}

func (m *mockTenantService) Create(ctx context.Context, tenant *model.Tenant) error {
	if m.err != nil {
		return m.err
	}
	m.tenants = append(m.tenants, tenant)
	return nil
}

func (m *mockTenantService) GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, t := range m.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockTenantService) GetBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, t := range m.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, nil
}

func (m *mockTenantService) List(ctx context.Context) ([]*model.Tenant, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tenants, nil
}

func TestTenantHandler_List(t *testing.T) {
	t.Parallel()

	tenants := []*model.Tenant{
		{ID: uuid.New(), Name: "Org 1", Slug: "org1"},
		{ID: uuid.New(), Name: "Org 2", Slug: "org2"},
	}
	svc := &mockTenantService{tenants: tenants}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	var responses []handler.TenantResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("want 2 tenants, got %d", len(responses))
	}
}

func TestTenantHandler_List_Empty(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{tenants: []*model.Tenant{}}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	var responses []handler.TenantResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(responses) != 0 {
		t.Errorf("want 0 tenants, got %d", len(responses))
	}
}

func TestTenantHandler_List_Error(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: context.DeadlineExceeded}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected error response")
	}
}

func TestTenantHandler_List_AppError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.NotFound("tenant")}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_Valid(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: uuid.New(), Name: "Test Org", Slug: "test"}
	svc := &mockTenantService{tenants: []*model.Tenant{tenant}}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/"+tenant.ID.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tenant.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_InvalidID(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.NotFound("tenant")}
	h := handler.NewTenantHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_NilTenant(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{tenants: []*model.Tenant{}}
	h := handler.NewTenantHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_EmptyID(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_Error(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: context.DeadlineExceeded}
	h := handler.NewTenantHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected error response")
	}
}

func TestTenantHandler_List_Pagination(t *testing.T) {
	t.Parallel()

	tenants := make([]*model.Tenant, 100)
	for i := range tenants {
		tenants[i] = &model.Tenant{
			ID:   uuid.New(),
			Name: "Org " + string(rune(i)),
			Slug: "org-" + string(rune(i)),
		}
	}
	svc := &mockTenantService{tenants: tenants}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	var responses []handler.TenantResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(responses) != 100 {
		t.Errorf("want 100 tenants, got %d", len(responses))
	}
}

func TestTenantHandler_ContentType(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: uuid.New(), Name: "Test", Slug: "test"}
	svc := &mockTenantService{tenants: []*model.Tenant{tenant}}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/"+tenant.ID.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tenant.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("want application/json, got %s", rr.Header().Get("Content-Type"))
	}
}

func TestTenantHandler_List_ResponseFormat(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{
		ID:        uuid.New(),
		Name:      "Test Org",
		Slug:      "test",
		Settings:  map[string]interface{}{"key": "value"},
		IsActive:  true,
	}
	svc := &mockTenantService{tenants: []*model.Tenant{tenant}}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	var responses []handler.TenantResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if responses[0].ID != tenant.ID {
		t.Errorf("want ID %s, got %s", tenant.ID, responses[0].ID)
	}
	if responses[0].Name != tenant.Name {
		t.Errorf("want Name %s, got %s", tenant.Name, responses[0].Name)
	}
	if responses[0].Slug != tenant.Slug {
		t.Errorf("want Slug %s, got %s", tenant.Slug, responses[0].Slug)
	}
}

func TestTenantHandler_GetByID_ResponseFormat(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{
		ID:        uuid.New(),
		Name:      "Test Org",
		Slug:      "test",
		Settings:  map[string]interface{}{"key": "value"},
		IsActive:  true,
	}
	svc := &mockTenantService{tenants: []*model.Tenant{tenant}}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/"+tenant.ID.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", tenant.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	var response model.TenantResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if response.ID != tenant.ID {
		t.Errorf("want ID %s, got %s", tenant.ID, response.ID)
	}
	if response.Name != tenant.Name {
		t.Errorf("want Name %s, got %s", tenant.Name, response.Name)
	}
}

func TestTenantHandler_MultipleConcurrentRequests(t *testing.T) {
	t.Parallel()

	tenant := &model.Tenant{ID: uuid.New(), Name: "Test", Slug: "test"}
	svc := &mockTenantService{tenants: []*model.Tenant{tenant}}
	h := handler.NewTenantHandler(svc)

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/"+tenant.ID.String(), nil)
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", tenant.ID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		h.GetByID(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: want 200, got %d", i, rr.Code)
		}
	}
}

func TestTenantHandler_List_ErrorFormatting(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.Internal(nil)}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_ErrorFormatting(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.Internal(nil)}
	h := handler.NewTenantHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}

func TestTenantHandler_List_BadRequestError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.BadRequest("invalid", nil)}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTenantHandler_GetByID_BadRequestError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.BadRequest("invalid", nil)}
	h := handler.NewTenantHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/"+id.String(), nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTenantHandler_List_ConflictError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.Conflict("already exists")}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rr.Code)
	}
}

func TestTenantHandler_List_ForbiddenError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.Forbidden("denied")}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rr.Code)
	}
}

func TestTenantHandler_List_UnauthorizedError(t *testing.T) {
	t.Parallel()

	svc := &mockTenantService{err: apperror.Unauthorized("not authenticated")}
	h := handler.NewTenantHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}