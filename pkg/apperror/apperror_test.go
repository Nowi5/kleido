package apperror_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nowi5/kleido/pkg/apperror"
)

// --- Constructor status codes ---

func TestConstructors_StatusCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      *apperror.AppError
		wantCode int
	}{
		{"NotFound", apperror.NotFound("user"), http.StatusNotFound},
		{"Unauthorized", apperror.Unauthorized("not allowed"), http.StatusUnauthorized},
		{"Forbidden", apperror.Forbidden("forbidden"), http.StatusForbidden},
		{"BadRequest", apperror.BadRequest("bad input", nil), http.StatusBadRequest},
		{"Conflict", apperror.Conflict("already exists"), http.StatusConflict},
		{"TooManyRequests", apperror.TooManyRequests(), http.StatusTooManyRequests},
		{"Internal", apperror.Internal(errors.New("db down")), http.StatusInternalServerError},
		{"New custom", apperror.New(http.StatusTeapot, "I'm a teapot", nil), http.StatusTeapot},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.err.Code != tc.wantCode {
				t.Errorf("Code: want %d, got %d", tc.wantCode, tc.err.Code)
			}
		})
	}
}

// --- Error() returns Message, not Internal.Error() ---

func TestAppError_ErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	internal := errors.New("raw db error")
	ae := apperror.New(http.StatusInternalServerError, "internal server error", internal)

	if ae.Error() != "internal server error" {
		t.Errorf("Error() should return Message, got %q", ae.Error())
	}
	if ae.Error() == internal.Error() {
		t.Error("Error() must not return the internal error text")
	}
}

// --- errors.Is / errors.As through wrapping ---

func TestErrorsAs_DirectAppError(t *testing.T) {
	t.Parallel()

	ae := apperror.NotFound("widget")
	var target *apperror.AppError
	if !errors.As(ae, &target) {
		t.Error("errors.As should find *AppError directly")
	}
	if target.Code != http.StatusNotFound {
		t.Errorf("Code: want 404, got %d", target.Code)
	}
}

func TestErrorsAs_TwoLevelsOfWrapping(t *testing.T) {
	t.Parallel()

	inner := apperror.NotFound("order")
	wrapped := fmt.Errorf("service layer: %w", inner)
	doubleWrapped := fmt.Errorf("handler layer: %w", wrapped)

	var target *apperror.AppError
	if !errors.As(doubleWrapped, &target) {
		t.Error("errors.As must unwrap through at least two levels of wrapping")
	}
	if target.Code != http.StatusNotFound {
		t.Errorf("Code: want 404, got %d", target.Code)
	}
}

// --- Predicates ---

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if !apperror.IsNotFound(apperror.NotFound("user")) {
		t.Error("IsNotFound should be true for NotFound error")
	}
	if apperror.IsNotFound(apperror.Unauthorized("x")) {
		t.Error("IsNotFound should be false for Unauthorized error")
	}
	if apperror.IsNotFound(errors.New("plain error")) {
		t.Error("IsNotFound should be false for plain error")
	}
}

func TestIsUnauthorized(t *testing.T) {
	t.Parallel()

	if !apperror.IsUnauthorized(apperror.Unauthorized("bad token")) {
		t.Error("IsUnauthorized should be true for Unauthorized error")
	}
	if apperror.IsUnauthorized(apperror.NotFound("x")) {
		t.Error("IsUnauthorized should be false for NotFound error")
	}
}

func TestIsConflict(t *testing.T) {
	t.Parallel()

	if !apperror.IsConflict(apperror.Conflict("email taken")) {
		t.Error("IsConflict should be true for Conflict error")
	}
	if apperror.IsConflict(apperror.BadRequest("bad", nil)) {
		t.Error("IsConflict should be false for BadRequest error")
	}
}

// --- WriteError ---

func TestWriteError_NotFound(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	apperror.WriteError(rr, apperror.NotFound("user"))

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !json.Valid([]byte(strings.TrimSpace(body))) {
		t.Errorf("body must be valid JSON; got %s", body)
	}
	if !strings.Contains(strings.ToLower(body), "not found") {
		t.Errorf("body must mention 'not found'; got %s", body)
	}
}

func TestWriteError_PlainError_Returns500(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	apperror.WriteError(rr, errors.New("boom: secret db error"))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "boom") {
		t.Errorf("response body must not leak the raw internal error; got %s", body)
	}
	if strings.Contains(body, "secret db error") {
		t.Errorf("response body must not leak the raw internal error; got %s", body)
	}
	if !strings.Contains(strings.ToLower(body), "internal server error") {
		t.Errorf("body must contain 'internal server error'; got %s", body)
	}
}

func TestWriteError_ContentType(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	apperror.WriteError(rr, apperror.BadRequest("bad input", nil))

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type must be application/json, got %q", ct)
	}
}

func TestNotFound_Message(t *testing.T) {
	t.Parallel()

	ae := apperror.NotFound("widget")
	if ae.Message != "widget not found" {
		t.Errorf("NotFound message: want %q, got %q", "widget not found", ae.Message)
	}
}
