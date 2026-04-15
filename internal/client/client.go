// Package client provides a typed HTTP client for the kleido API.
// It does not import internal/service, internal/handler, internal/repository,
// or pkg/apperror — it is safe to use from the CLI binary.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiErrorBody mirrors the JSON error envelope returned by the API.
type apiErrorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client is the root API client. Construct with New().
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	Auth       *AuthService
	Users      *UsersService
}

// New constructs a Client. baseURL should not have a trailing slash.
// token is injected into every request as "Authorization: Bearer <token>".
// If token is empty, the Authorization header is omitted.
func New(baseURL, token string) *Client {
	c := &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	c.Auth = &AuthService{c: c}
	c.Users = &UsersService{c: c}
	return c
}

// do executes an HTTP request and decodes the JSON response into out.
// If out is nil, the response body is discarded.
// HTTP 4xx/5xx responses are decoded as {"error":{"code":N,"message":"M"}}
// and returned as a Go error. Network errors are wrapped with "request failed: ".
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("request failed: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errBody apiErrorBody
		if decErr := json.NewDecoder(resp.Body).Decode(&errBody); decErr != nil || errBody.Error.Message == "" {
			return fmt.Errorf("API %d: unexpected response", resp.StatusCode)
		}
		return fmt.Errorf("API %d: %s", resp.StatusCode, errBody.Error.Message)
	}

	if out != nil {
		if decErr := json.NewDecoder(resp.Body).Decode(out); decErr != nil {
			return fmt.Errorf("request failed: decode response: %w", decErr)
		}
	}

	return nil
}
