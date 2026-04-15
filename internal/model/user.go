// Package model defines the core domain types for the kleido application.
package model

import (
	"time"

	"github.com/google/uuid"
)

// User is the persistent domain entity for an application user.
type User struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	Email        string    `db:"email"         json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         string    `db:"role"          json:"role"`
	IsActive     bool      `db:"is_active"     json:"is_active"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// UserResponse is the safe public shape — no password hash, used in handlers.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateUserRequest carries the fields a caller may change on a user.
// All fields are pointers — nil means "do not change this field" (patch semantics).
type UpdateUserRequest struct {
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

// ToResponse converts a User to the safe public-facing UserResponse shape.
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID: u.ID, Email: u.Email, Role: u.Role,
		IsActive: u.IsActive, CreatedAt: u.CreatedAt,
	}
}
