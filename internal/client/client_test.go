package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kleido/internal/client"
)

// writeJSON writes v as JSON with the given HTTP status code.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("writeJSON: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()
	want := client.LoginResponse{
		AccessToken: "tok-abc",
		ExpiresAt:   time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "")
	got, err := c.Auth.Login(context.Background(), client.LoginRequest{Email: "a@b.com", Password: "secret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.AccessToken != want.AccessToken {
		t.Errorf("AccessToken: want %q, got %q", want.AccessToken, got.AccessToken)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"code": 401, "message": "invalid credentials"},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "")
	_, err := c.Auth.Login(context.Background(), client.LoginRequest{Email: "a@b.com", Password: "wrong"})
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestLogout_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/logout" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok-abc")
	if err := c.Auth.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestRegister_Success(t *testing.T) {
	t.Parallel()
	want := client.UserResponse{
		ID:    "00000000-0000-0000-0000-000000000001",
		Email: "new@example.com",
		Role:  "user",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/register" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusCreated, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "")
	got, err := c.Auth.Register(context.Background(), "new@example.com", "Password1!")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.Email != want.Email {
		t.Errorf("email: want %q, got %q", want.Email, got.Email)
	}
}

func TestUsersMe_Success(t *testing.T) {
	t.Parallel()
	want := client.UserResponse{ID: "uid-1", Email: "me@example.com", Role: "user"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/users/me" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	got, err := c.Users.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if got.Email != want.Email {
		t.Errorf("email: want %q, got %q", want.Email, got.Email)
	}
}

func TestUsersGet_Success(t *testing.T) {
	t.Parallel()
	id := "00000000-0000-0000-0000-000000000002"
	want := client.UserResponse{ID: id, Email: "user@example.com", Role: "user"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/users/"+id {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	got, err := c.Users.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID: want %q, got %q", id, got.ID)
	}
}

func TestUsersList_Success(t *testing.T) {
	t.Parallel()
	want := client.ListUsersResponse{
		Data:    []*client.UserResponse{{ID: "u1", Email: "a@a.com"}},
		Total:   1,
		Page:    1,
		PerPage: 20,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/users" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	got, err := c.Users.List(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Total != want.Total {
		t.Errorf("Total: want %d, got %d", want.Total, got.Total)
	}
}

func TestUsersUpdate_Success(t *testing.T) {
	t.Parallel()
	id := "00000000-0000-0000-0000-000000000003"
	newEmail := "updated@example.com"
	want := client.UserResponse{ID: id, Email: newEmail, Role: "user"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/users/"+id {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	got, err := c.Users.Update(context.Background(), id, client.UpdateUserRequest{Email: &newEmail})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Email != newEmail {
		t.Errorf("email: want %q, got %q", newEmail, got.Email)
	}
}

func TestUsersDelete_Success(t *testing.T) {
	t.Parallel()
	id := "00000000-0000-0000-0000-000000000004"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/users/"+id {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(srv.URL, "tok")
	if err := c.Users.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestDo_APIError_FormatsMessage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusForbidden, map[string]any{
			"error": map[string]any{"code": 403, "message": "forbidden"},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "")
	err := c.Users.Delete(context.Background(), "any-id")
	if err == nil {
		t.Fatal("expected error for 403")
	}
	const want = "API 403: forbidden"
	if err.Error() != want {
		t.Errorf("error: want %q, got %q", want, err.Error())
	}
}

func TestDo_MalformedErrorBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("not json")); err != nil {
			t.Errorf("write: %v", err)
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL, "")
	_, err := c.Users.Me(context.Background())
	if err == nil {
		t.Fatal("expected error for 500")
	}
	const want = "API 500: unexpected response"
	if err.Error() != want {
		t.Errorf("error: want %q, got %q", want, err.Error())
	}
}
