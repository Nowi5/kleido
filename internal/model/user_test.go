package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUser_ToResponse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tenantID := uuid.New()

	tests := []struct {
		name     string
		user     *User
		expected *UserResponse
	}{
		{
			name: "full user",
			user: &User{
				ID:           uuid.New(),
				TenantID:     &tenantID,
				Email:        "test@example.com",
				PasswordHash: "hashedpassword",
				Role:         "user",
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			expected: &UserResponse{
				Email:     "test@example.com",
				Role:      "user",
				IsActive:  true,
				CreatedAt: now,
			},
		},
		{
			name: "user without tenant",
			user: &User{
				ID:           uuid.New(),
				TenantID:     nil,
				Email:        "admin@example.com",
				PasswordHash: "hashedpassword",
				Role:         "admin",
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			expected: &UserResponse{
				Email:     "admin@example.com",
				Role:      "admin",
				IsActive:  true,
				CreatedAt: now,
			},
		},
		{
			name: "inactive user",
			user: &User{
				ID:           uuid.New(),
				Email:        "inactive@example.com",
				PasswordHash: "hashedpassword",
				Role:         "user",
				IsActive:     false,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			expected: &UserResponse{
				Email:     "inactive@example.com",
				Role:      "user",
				IsActive:  false,
				CreatedAt: now,
			},
		},
		{
			name: "moderator role",
			user: &User{
				ID:           uuid.New(),
				Email:        "mod@example.com",
				PasswordHash: "hashedpassword",
				Role:         "moderator",
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			expected: &UserResponse{
				Email:     "mod@example.com",
				Role:      "moderator",
				IsActive:  true,
				CreatedAt: now,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.user.ToResponse()

			if result.ID != tc.user.ID {
				t.Errorf("ID mismatch: got %v, want %v", result.ID, tc.user.ID)
			}
			if result.Email != tc.expected.Email {
				t.Errorf("Email mismatch: got %v, want %v", result.Email, tc.expected.Email)
			}
			if result.Role != tc.expected.Role {
				t.Errorf("Role mismatch: got %v, want %v", result.Role, tc.expected.Role)
			}
			if result.IsActive != tc.expected.IsActive {
				t.Errorf("IsActive mismatch: got %v, want %v", result.IsActive, tc.expected.IsActive)
			}
			if result.CreatedAt != tc.expected.CreatedAt {
				t.Errorf("CreatedAt mismatch: got %v, want %v", result.CreatedAt, tc.expected.CreatedAt)
			}
		})
	}
}

func TestUser_ToResponse_IDMatches(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	user := &User{
		ID:        userID,
		Email:     "test@example.com",
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	response := user.ToResponse()

	if response.ID != userID {
		t.Errorf("ToResponse() should preserve ID: got %v, want %v", response.ID, userID)
	}
}

func TestUser_ToResponse_EmptyFields(t *testing.T) {
	t.Parallel()

	user := &User{
		ID:        uuid.Nil,
		Email:     "",
		Role:      "",
		IsActive:  false,
		CreatedAt: time.Time{},
	}

	response := user.ToResponse()

	if response.ID != uuid.Nil {
		t.Errorf("ID should be Nil UUID")
	}
	if response.Email != "" {
		t.Errorf("Email should be empty")
	}
	if response.Role != "" {
		t.Errorf("Role should be empty")
	}
}

func TestUser_StructFields(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	now := time.Now()

	user := User{
		ID:           uuid.New(),
		TenantID:     &tenantID,
		Email:        "user@example.com",
		PasswordHash: "somehash",
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if user.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if user.TenantID == nil {
		t.Error("TenantID should not be nil")
	}
	if user.Email == "" {
		t.Error("Email should not be empty")
	}
	if user.PasswordHash == "" {
		t.Error("PasswordHash should not be empty")
	}
	if user.Role == "" {
		t.Error("Role should not be empty")
	}
	if !user.IsActive {
		t.Error("IsActive should be true")
	}
	if user.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestUser_NilTenantID(t *testing.T) {
	t.Parallel()

	user := User{
		ID:       uuid.New(),
		TenantID: nil,
		Email:    "user@example.com",
		Role:     "user",
		IsActive: true,
	}

	if user.TenantID != nil {
		t.Error("TenantID should be nil for this test")
	}
}

func TestUserResponse_JSONFields(t *testing.T) {
	t.Parallel()

	resp := UserResponse{
		ID:        uuid.New(),
		Email:     "test@example.com",
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if resp.ID == uuid.Nil {
		t.Error("ID should be set")
	}
	if resp.Email == "" {
		t.Error("Email should be set")
	}
	if resp.Role == "" {
		t.Error("Role should be set")
	}
}

func TestUpdateUserRequest_PatchSemantics(t *testing.T) {
	t.Parallel()

	email := "newemail@example.com"
	role := "admin"
	active := false

	req := &UpdateUserRequest{
		Email:    &email,
		Role:     &role,
		IsActive: &active,
	}

	if req.Email == nil {
		t.Error("Email should not be nil")
	}
	if *req.Email != "newemail@example.com" {
		t.Errorf("Email value mismatch: got %v", *req.Email)
	}
	if req.Role == nil {
		t.Error("Role should not be nil")
	}
	if *req.Role != "admin" {
		t.Errorf("Role value mismatch: got %v", *req.Role)
	}
	if req.IsActive == nil {
		t.Error("IsActive should not be nil")
	}
	if *req.IsActive != false {
		t.Errorf("IsActive value mismatch: got %v", *req.IsActive)
	}
}

func TestUpdateUserRequest_NilMeansNoChange(t *testing.T) {
	t.Parallel()

	req := &UpdateUserRequest{
		Email:    nil,
		Role:     nil,
		IsActive: nil,
	}

	if req.Email != nil {
		t.Error("Email should be nil (no change)")
	}
	if req.Role != nil {
		t.Error("Role should be nil (no change)")
	}
	if req.IsActive != nil {
		t.Error("IsActive should be nil (no change)")
	}
}