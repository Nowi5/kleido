package client

import (
	"context"
	"time"
)

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the body returned by POST /auth/login.
type LoginResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// AuthService wraps authentication API calls.
type AuthService struct{ c *Client }

// Login authenticates with email and password, returning an access token.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	var out LoginResponse
	if err := s.c.do(ctx, "POST", "/api/v1/auth/login", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Logout revokes the current session. Requires a valid Bearer token set on the client.
func (s *AuthService) Logout(ctx context.Context) error {
	return s.c.do(ctx, "POST", "/api/v1/auth/logout", nil, nil)
}

// Register creates a new user account and returns the created user's profile.
// The new account is assigned the "user" role.
func (s *AuthService) Register(ctx context.Context, email, password string) (*UserResponse, error) {
	body := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}
	var out UserResponse
	if err := s.c.do(ctx, "POST", "/api/v1/auth/register", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
