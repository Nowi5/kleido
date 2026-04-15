package client

import (
	"context"
	"fmt"
	"time"
)

// UserResponse mirrors model.UserResponse returned by the API.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// ListUsersResponse mirrors handler.ListUsersResponse.
type ListUsersResponse struct {
	Data    []*UserResponse `json:"data"`
	Total   int64           `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}

// UpdateUserRequest carries patch fields for PUT /users/{id}.
// All fields are optional — nil means "do not change this field".
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty"`
	Role     *string `json:"role,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// UsersService wraps user-related API calls.
type UsersService struct{ c *Client }

// Me returns the authenticated user's own profile.
func (s *UsersService) Me(ctx context.Context) (*UserResponse, error) {
	var out UserResponse
	if err := s.c.do(ctx, "GET", "/api/v1/users/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns a single user by UUID string.
func (s *UsersService) Get(ctx context.Context, id string) (*UserResponse, error) {
	var out UserResponse
	if err := s.c.do(ctx, "GET", "/api/v1/users/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns a paginated list of all users (admin only).
func (s *UsersService) List(ctx context.Context, page, perPage int) (*ListUsersResponse, error) {
	path := fmt.Sprintf("/api/v1/users?page=%d&per_page=%d", page, perPage)
	var out ListUsersResponse
	if err := s.c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update partially updates a user (patch semantics — nil fields are unchanged).
func (s *UsersService) Update(ctx context.Context, id string, req UpdateUserRequest) (*UserResponse, error) {
	var out UserResponse
	if err := s.c.do(ctx, "PUT", "/api/v1/users/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete soft-deletes a user (sets is_active = false). Admin only.
func (s *UsersService) Delete(ctx context.Context, id string) error {
	return s.c.do(ctx, "DELETE", "/api/v1/users/"+id, nil, nil)
}
