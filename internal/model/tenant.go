package model

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID              `db:"id"         json:"id"`
	Name      string                 `db:"name"       json:"name"`
	Slug      string                 `db:"slug"       json:"slug"`
	Settings  map[string]interface{} `db:"settings"   json:"settings"`
	IsActive  bool                   `db:"is_active"  json:"is_active"`
	CreatedAt time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt time.Time              `db:"updated_at" json:"updated_at"`
}

type TenantResponse struct {
	ID        uuid.UUID              `json:"id"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	Settings  map[string]interface{} `json:"settings"`
	IsActive  bool                   `json:"is_active"`
	CreatedAt time.Time              `json:"created_at"`
}

func (t *Tenant) ToResponse() *TenantResponse {
	return &TenantResponse{
		ID:        t.ID,
		Name:      t.Name,
		Slug:      t.Slug,
		Settings:  t.Settings,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt,
	}
}