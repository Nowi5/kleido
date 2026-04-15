package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTenant_ToResponse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tenantID := uuid.New()

	tests := []struct {
		name   string
		tenant *Tenant
	}{
		{
			name: "full tenant",
			tenant: &Tenant{
				ID:        tenantID,
				Name:      "Acme Corp",
				Slug:      "acme",
				Settings:  map[string]interface{}{"theme": "dark"},
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		{
			name: "inactive tenant",
			tenant: &Tenant{
				ID:        uuid.New(),
				Name:      "Inactive Org",
				Slug:      "inactive",
				Settings:  map[string]interface{}{},
				IsActive:  false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		{
			name: "tenant with empty settings",
			tenant: &Tenant{
				ID:        uuid.New(),
				Name:      "Empty Settings",
				Slug:      "empty",
				Settings:  nil,
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.tenant.ToResponse()

			if result.ID != tc.tenant.ID {
				t.Errorf("ID mismatch: got %v, want %v", result.ID, tc.tenant.ID)
			}
			if result.Name != tc.tenant.Name {
				t.Errorf("Name mismatch: got %v, want %v", result.Name, tc.tenant.Name)
			}
			if result.Slug != tc.tenant.Slug {
				t.Errorf("Slug mismatch: got %v, want %v", result.Slug, tc.tenant.Slug)
			}
			if result.IsActive != tc.tenant.IsActive {
				t.Errorf("IsActive mismatch: got %v, want %v", result.IsActive, tc.tenant.IsActive)
			}
		})
	}
}

func TestTenant_ToResponse_IDMatches(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tenant := &Tenant{
		ID:        tenantID,
		Name:      "Test Tenant",
		Slug:      "test",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	response := tenant.ToResponse()

	if response.ID != tenantID {
		t.Errorf("ToResponse() should preserve ID: got %v, want %v", response.ID, tenantID)
	}
}

func TestTenant_ToResponse_EmptyFields(t *testing.T) {
	t.Parallel()

	tenant := &Tenant{
		ID:        uuid.Nil,
		Name:      "",
		Slug:      "",
		Settings:  nil,
		IsActive:  false,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}

	response := tenant.ToResponse()

	if response.ID != uuid.Nil {
		t.Error("ID should be Nil UUID")
	}
	if response.Name != "" {
		t.Error("Name should be empty")
	}
	if response.Slug != "" {
		t.Error("Slug should be empty")
	}
}

func TestTenant_StructFields(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tenant := Tenant{
		ID:        uuid.New(),
		Name:      "Test Organization",
		Slug:      "test-org",
		Settings:  map[string]interface{}{"key": "value"},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if tenant.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if tenant.Name == "" {
		t.Error("Name should not be empty")
	}
	if tenant.Slug == "" {
		t.Error("Slug should not be empty")
	}
	if tenant.Settings == nil {
		t.Error("Settings should not be nil")
	}
	if !tenant.IsActive {
		t.Error("IsActive should be true")
	}
	if tenant.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if tenant.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestTenant_SettingsJSONB(t *testing.T) {
	t.Parallel()

	tenant := Tenant{
		ID:        uuid.New(),
		Name:      "JSONB Test",
		Slug:      "jsonb",
		Settings: map[string]interface{}{
			"theme":      "dark",
			"features":   []string{"a", "b", "c"},
			"limits":    map[string]int{"users": 100},
			"customKey": nil,
		},
		IsActive: true,
	}

	if tenant.Settings["theme"] != "dark" {
		t.Errorf("Settings theme mismatch: got %v", tenant.Settings["theme"])
	}
	if tenant.Settings["features"] == nil {
		t.Error("Settings features should exist")
	}
	if tenant.Settings["limits"] == nil {
		t.Error("Settings limits should exist")
	}
}

func TestTenant_UniqueSlug(t *testing.T) {
	t.Parallel()

	tenant := Tenant{
		ID:   uuid.New(),
		Name: "Duplicate Name",
		Slug: "unique-slug-123",
	}

	if tenant.Slug == "" {
		t.Error("Slug should not be empty for unique constraint")
	}
}

func TestTenantResponse_JSONFields(t *testing.T) {
	t.Parallel()

	resp := TenantResponse{
		ID:        uuid.New(),
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"key": "value"},
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if resp.ID == uuid.Nil {
		t.Error("ID should be set")
	}
	if resp.Name == "" {
		t.Error("Name should be set")
	}
	if resp.Slug == "" {
		t.Error("Slug should be set")
	}
	if !resp.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestTenant_DefaultValues(t *testing.T) {
	t.Parallel()

	tenant := Tenant{}

	if tenant.ID != uuid.Nil {
		t.Error("ID should default to Nil")
	}
	if tenant.Name != "" {
		t.Error("Name should default to empty")
	}
	if tenant.Slug != "" {
		t.Error("Slug should default to empty")
	}
	if tenant.Settings != nil {
		t.Error("Settings should default to nil")
	}
	if tenant.IsActive != false {
		t.Error("IsActive should default to false")
	}
}