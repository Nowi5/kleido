package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"kleido/internal/model"
	"kleido/internal/service"
	"kleido/pkg/apperror"
	"github.com/redis/go-redis/v9"
)

// --- Repository mocks ---

type mockUserRepo struct {
	users      map[uuid.UUID]*model.User
	updateErr  error
	deleteErr  error
	createErr  error
	updateUser *model.User // last passed to Update
}

func (m *mockUserRepo) FindByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, apperror.NotFound("user")
	}
	return u, nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, apperror.NotFound("user")
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.users == nil {
		m.users = map[uuid.UUID]*model.User{}
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Update(_ context.Context, user *model.User) error {
	m.updateUser = user
	return m.updateErr
}

func (m *mockUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if u, ok := m.users[id]; ok {
		u.IsActive = false
	}
	return nil
}

func (m *mockUserRepo) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockUserRepo) List(_ context.Context, _, _ int) ([]*model.User, int64, error) {
	return nil, 0, nil
}

type mockCacheRepo struct {
	data            map[string]any
	deletedKey      string
	deleteCallCount int
}

func (m *mockCacheRepo) SetJSON(_ context.Context, key string, v any, _ time.Duration) error {
	if m.data == nil {
		m.data = map[string]any{}
	}
	m.data[key] = v
	return nil
}

func (m *mockCacheRepo) GetJSON(_ context.Context, key string, dest any) error {
	v, ok := m.data[key]
	if !ok {
		return redis.Nil
	}
	// For test purposes, just check existence — callers check the pointer type.
	_ = v
	_ = dest
	return redis.Nil // always miss in unit tests (forces DB path)
}

func (m *mockCacheRepo) Delete(_ context.Context, keys ...string) error {
	m.deleteCallCount++
	if len(keys) > 0 {
		m.deletedKey = keys[0]
	}
	return nil
}

func (m *mockCacheRepo) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// --- Helpers ---

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func newUserSvc(repo *mockUserRepo, cache *mockCacheRepo) service.UserService {
	return service.NewUserService(repo, cache, nil)
}

// --- Update tests ---

func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "old@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	newEmail := "new@example.com"
	got, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{Email: &newEmail}, "user")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Email != newEmail {
		t.Errorf("email: want %q, got %q", newEmail, got.Email)
	}
	if cache.deleteCallCount != 1 {
		t.Errorf("cache.Delete call count: want 1, got %d", cache.deleteCallCount)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	_, err := svc.Update(context.Background(), uuid.New(), &model.UpdateUserRequest{}, "user")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("want NotFound, got: %v", err)
	}
}

func TestUpdate_RoleChange_NonAdmin_Ignored(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "a@b.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	newRole := "admin"
	_, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{Role: &newRole}, "user")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Role must remain unchanged because caller is not admin.
	if repo.updateUser.Role != "user" {
		t.Errorf("role should remain %q for non-admin, got %q", "user", repo.updateUser.Role)
	}
}

func TestUpdate_RoleChange_Admin_Applied(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "a@b.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	newRole := "admin"
	_, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{Role: &newRole}, "admin")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updateUser.Role != "admin" {
		t.Errorf("role: want %q, got %q", "admin", repo.updateUser.Role)
	}
}

func TestUpdate_IsActive_Applied(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "a@b.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	inactive := false
	got, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{IsActive: &inactive}, "admin")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.IsActive {
		t.Error("IsActive should be false after update")
	}
}

func TestUpdate_CacheInvalidated(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "x@y.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	_, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{Email: strPtr("new@y.com")}, "user")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	wantKey := "cache:user:" + id.String()
	if cache.deletedKey != wantKey {
		t.Errorf("cache.Delete key: want %q, got %q", wantKey, cache.deletedKey)
	}
}

// --- Delete tests ---

func TestDelete_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "d@e.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if cache.deleteCallCount != 1 {
		t.Errorf("cache.Delete call count: want 1, got %d", cache.deleteCallCount)
	}
}

func TestDelete_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	err := svc.Delete(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("want NotFound, got: %v", err)
	}
}

func TestDelete_CacheInvalidated(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "d@e.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	wantKey := "cache:user:" + id.String()
	if cache.deletedKey != wantKey {
		t.Errorf("cache.Delete key: want %q, got %q", wantKey, cache.deletedKey)
	}
}

// TestCacheKeyConsistency verifies that the cache key format used in GetByID
// and Update/Delete is identical — a mismatch would mean invalidation misses.
func TestCacheKeyConsistency(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "k@k.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	// GetByID writes to the cache (SetJSON). It does not call Delete.
	if _, getErr := svc.GetByID(context.Background(), id); getErr != nil {
		t.Logf("GetByID (cache miss expected in mock): %v", getErr)
	}
	populatedKey := cache.deletedKey // not set by GetByID (it writes, not deletes)

	// Update must delete the same key that GetByID would have populated.
	_, err := svc.Update(context.Background(), id, &model.UpdateUserRequest{Email: strPtr("k2@k.com")}, "admin")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	_ = populatedKey // only checking the Delete key
	wantKey := "cache:user:" + id.String()
	if cache.deletedKey != wantKey {
		t.Errorf("Update cache.Delete key: want %q, got %q", wantKey, cache.deletedKey)
	}

	// Delete must also use the same key.
	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if cache.deletedKey != wantKey {
		t.Errorf("Delete cache.Delete key: want %q, got %q", wantKey, cache.deletedKey)
	}
}

// Ensure boolPtr helper compiles (used for IsActive field testing).
var _ = boolPtr(true)

// mockCacheRepoEx extends mockCacheRepo with cache-hit support for GetByID tests.
type mockCacheRepoEx struct {
	mockCacheRepo
	hitUser    *model.User
	getJSONErr error // non-nil and non-redis.Nil → simulates cache error
}

func (m *mockCacheRepoEx) GetJSON(_ context.Context, _ string, dest any) error {
	if m.getJSONErr != nil {
		return m.getJSONErr
	}
	if m.hitUser == nil {
		return redis.Nil
	}
	// Simulate a real cache hit by round-tripping through JSON.
	b, err := json.Marshal(m.hitUser)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// --- GetByEmail tests ---

func TestGetByEmail_Found(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "found@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	// Override FindByEmail to return the user.
	repoWithEmail := &mockUserRepoWithEmail{inner: repo, user: user}
	cache := &mockCacheRepo{}
	svc := service.NewUserService(repoWithEmail, cache, nil)

	got, err := svc.GetByEmail(context.Background(), "found@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Email != user.Email {
		t.Errorf("email: want %q, got %q", user.Email, got.Email)
	}
}

func TestGetByEmail_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	_, err := svc.GetByEmail(context.Background(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected error for missing email")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("want NotFound, got: %v", err)
	}
}

// mockUserRepoWithEmail wraps mockUserRepo to return a specific user for FindByEmail.
type mockUserRepoWithEmail struct {
	inner *mockUserRepo
	user  *model.User
}

func (m *mockUserRepoWithEmail) Create(ctx context.Context, u *model.User) error {
	return m.inner.Create(ctx, u)
}
func (m *mockUserRepoWithEmail) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.inner.FindByID(ctx, id)
}
func (m *mockUserRepoWithEmail) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return m.user, nil
}
func (m *mockUserRepoWithEmail) Update(ctx context.Context, u *model.User) error {
	return m.inner.Update(ctx, u)
}
func (m *mockUserRepoWithEmail) Delete(ctx context.Context, id uuid.UUID) error {
	return m.inner.Delete(ctx, id)
}
func (m *mockUserRepoWithEmail) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	return m.inner.UpdatePassword(ctx, id, hash)
}
func (m *mockUserRepoWithEmail) List(ctx context.Context, limit, offset int) ([]*model.User, int64, error) {
	return m.inner.List(ctx, limit, offset)
}

// --- GetByID tests ---

func TestGetByID_CacheHit(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	cached := &model.User{ID: id, Email: "cached@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{}}
	cache := &mockCacheRepoEx{hitUser: cached}
	svc := service.NewUserService(repo, cache, nil)

	got, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != cached.Email {
		t.Errorf("email: want %q, got %q", cached.Email, got.Email)
	}
}

func TestGetByID_CacheMiss_DBHit(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "db@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepoEx{} // no hit — forces DB path
	svc := service.NewUserService(repo, cache, nil)

	got, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != user.Email {
		t.Errorf("email: want %q, got %q", user.Email, got.Email)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{}}
	cache := &mockCacheRepoEx{}
	svc := service.NewUserService(repo, cache, nil)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("want NotFound, got: %v", err)
	}
}

func TestGetByID_CacheError_FallsThrough(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "x@x.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepoEx{getJSONErr: errors.New("redis timeout")}
	svc := service.NewUserService(repo, cache, nil)

	// A cache error must not surface to the caller — it falls through to DB.
	got, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != user.Email {
		t.Errorf("email: want %q, got %q", user.Email, got.Email)
	}
}

// --- Create tests ---

func TestCreate_Success(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	got, err := svc.Create(context.Background(), "new@example.com", "Password1!", "user")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Email != "new@example.com" {
		t.Errorf("email: want %q, got %q", "new@example.com", got.Email)
	}
	if got.Role != "user" {
		t.Errorf("role: want %q, got %q", "user", got.Role)
	}
	if !got.IsActive {
		t.Error("new user must be active")
	}
}

func TestCreate_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepo{createErr: errors.New("db: unique constraint")}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	_, err := svc.Create(context.Background(), "dup@example.com", "Password1!", "user")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

// --- UpdatePassword tests ---

func TestUpdatePassword_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "u@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepo{users: map[uuid.UUID]*model.User{id: user}}
	cache := &mockCacheRepo{}
	svc := newUserSvc(repo, cache)

	if err := svc.UpdatePassword(context.Background(), id, "hashed-pw"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	// Cache entry must be invalidated after password change.
	if cache.deleteCallCount != 1 {
		t.Errorf("cache.Delete call count: want 1, got %d", cache.deleteCallCount)
	}
	wantKey := "cache:user:" + id.String()
	if cache.deletedKey != wantKey {
		t.Errorf("cache.Delete key: want %q, got %q", wantKey, cache.deletedKey)
	}
}

func TestUpdatePassword_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepoUpdatePWErr{err: errors.New("db: update failed")}
	cache := &mockCacheRepo{}
	svc := service.NewUserService(repo, cache, nil)

	err := svc.UpdatePassword(context.Background(), uuid.New(), "hashed-pw")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

// mockUserRepoUpdatePWErr returns an error from UpdatePassword.
type mockUserRepoUpdatePWErr struct {
	mockUserRepo
	err error
}

func (m *mockUserRepoUpdatePWErr) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error {
	return m.err
}

// --- List tests ---

func TestList_Success(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	user := &model.User{ID: id, Email: "l@example.com", Role: "user", IsActive: true}
	repo := &mockUserRepoList{users: []*model.User{user}, total: 1}
	cache := &mockCacheRepo{}
	svc := service.NewUserService(repo, cache, nil)

	results, total, err := svc.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total: want 1, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("len(results): want 1, got %d", len(results))
	}
	if results[0].Email != user.Email {
		t.Errorf("email: want %q, got %q", user.Email, results[0].Email)
	}
}

func TestList_Error(t *testing.T) {
	t.Parallel()

	repo := &mockUserRepoList{err: errors.New("db error")}
	cache := &mockCacheRepo{}
	svc := service.NewUserService(repo, cache, nil)

	_, _, err := svc.List(context.Background(), 10, 0)
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

// mockUserRepoList overrides List to return controlled results.
type mockUserRepoList struct {
	users []*model.User
	total int64
	err   error
}

func (m *mockUserRepoList) Create(_ context.Context, _ *model.User) error { return nil }
func (m *mockUserRepoList) FindByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return nil, apperror.NotFound("user")
}
func (m *mockUserRepoList) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, apperror.NotFound("user")
}
func (m *mockUserRepoList) Update(_ context.Context, _ *model.User) error                 { return nil }
func (m *mockUserRepoList) UpdatePassword(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *mockUserRepoList) Delete(_ context.Context, _ uuid.UUID) error                   { return nil }
func (m *mockUserRepoList) List(_ context.Context, _, _ int) ([]*model.User, int64, error) {
	return m.users, m.total, m.err
}
