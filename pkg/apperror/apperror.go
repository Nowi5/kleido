// Package apperror provides typed HTTP-aware application errors and helpers
// for writing JSON error responses without leaking internal details.
package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// AppError is the single error type that flows across all application layers.
// Handlers call WriteError which reads Code and Message from this struct.
type AppError struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Internal error  `json:"-"`
}

// Error implements the error interface. It returns the user-facing message,
// not the internal error.
func (e *AppError) Error() string {
	return e.Message
}

// Unwrap returns the wrapped internal error so errors.Is / errors.As work.
func (e *AppError) Unwrap() error {
	return e.Internal
}

// ErrorResponse is the JSON envelope sent to HTTP clients.
// It is exported for Swagger annotation use.
type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// --- Constructors ---

// New creates an AppError with an explicit HTTP status code.
func New(code int, msg string, internal error) *AppError {
	return &AppError{Code: code, Message: msg, Internal: internal}
}

// NotFound creates a 404 error for a missing resource.
func NotFound(resource string) *AppError {
	return New(http.StatusNotFound, resource+" not found", nil)
}

// Unauthorized creates a 401 error.
func Unauthorized(msg string) *AppError {
	return New(http.StatusUnauthorized, msg, nil)
}

// Forbidden creates a 403 error.
func Forbidden(msg string) *AppError {
	return New(http.StatusForbidden, msg, nil)
}

// BadRequest creates a 400 error with an optional internal cause.
func BadRequest(msg string, internal error) *AppError {
	return New(http.StatusBadRequest, msg, internal)
}

// Conflict creates a 409 error.
func Conflict(msg string) *AppError {
	return New(http.StatusConflict, msg, nil)
}

// TooManyRequests creates a 429 error.
func TooManyRequests() *AppError {
	return New(http.StatusTooManyRequests, "too many requests", nil)
}

// Internal creates a 500 error wrapping an internal cause. The internal error
// is never surfaced to HTTP clients.
func Internal(internal error) *AppError {
	return New(http.StatusInternalServerError, "internal server error", internal)
}

// --- Predicates ---

// IsNotFound reports whether err is (or wraps) a 404 AppError.
func IsNotFound(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Code == http.StatusNotFound
}

// IsUnauthorized reports whether err is (or wraps) a 401 AppError.
func IsUnauthorized(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Code == http.StatusUnauthorized
}

// IsConflict reports whether err is (or wraps) a 409 AppError.
func IsConflict(err error) bool {
	var ae *AppError
	return errors.As(err, &ae) && ae.Code == http.StatusConflict
}

// --- HTTP ---

// WriteError writes the appropriate HTTP status code and a JSON error body.
// If err is (or wraps) an *AppError, its Code and Message are used.
// Otherwise a 500 "internal server error" is returned and the raw error message
// is NOT exposed to the caller.
func WriteError(w http.ResponseWriter, err error) {
	var ae *AppError
	if !errors.As(err, &ae) {
		ae = Internal(err)
	}

	var resp ErrorResponse
	resp.Error.Code = ae.Code
	resp.Error.Message = ae.Message

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.Code)
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck // cannot handle encode error after WriteHeader
}
