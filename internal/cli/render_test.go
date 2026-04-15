package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"kleido/internal/client"
)

var testUser = &client.UserResponse{
	ID:        "00000000-0000-0000-0000-000000000001",
	Email:     "test@example.com",
	Role:      "user",
	IsActive:  true,
	CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
}

func TestRenderOutput_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := renderOutput("json", testUser, &buf); err != nil {
		t.Fatalf("renderOutput json: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"email"`) {
		t.Errorf("json output missing email field: %q", out)
	}
	if !strings.Contains(out, "test@example.com") {
		t.Errorf("json output missing email value: %q", out)
	}
}

func TestRenderOutput_YAML(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := renderOutput("yaml", testUser, &buf); err != nil {
		t.Fatalf("renderOutput yaml: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "email:") {
		t.Errorf("yaml output missing email key: %q", out)
	}
	if !strings.Contains(out, "test@example.com") {
		t.Errorf("yaml output missing email value: %q", out)
	}
}

func TestRenderOutput_Table_User(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := renderOutput("table", testUser, &buf); err != nil {
		t.Fatalf("renderOutput table user: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "EMAIL") {
		t.Errorf("table output missing EMAIL header: %q", out)
	}
	if !strings.Contains(out, "test@example.com") {
		t.Errorf("table output missing email value: %q", out)
	}
}

func TestRenderOutput_Table_UsersList(t *testing.T) {
	t.Parallel()
	resp := &client.ListUsersResponse{
		Data:    []*client.UserResponse{testUser},
		Total:   1,
		Page:    1,
		PerPage: 20,
	}
	var buf bytes.Buffer
	if err := renderOutput("table", resp, &buf); err != nil {
		t.Fatalf("renderOutput table list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "EMAIL") {
		t.Errorf("table list missing EMAIL header: %q", out)
	}
	if !strings.Contains(out, "test@example.com") {
		t.Errorf("table list missing email value: %q", out)
	}
}

func TestRenderOutput_Table_DefaultFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// "table" is the default and any unknown format also falls back to table.
	if err := renderOutput("table", testUser, &buf); err != nil {
		t.Fatalf("renderOutput: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty table output")
	}
}

func TestRenderOutput_Table_UnknownType_FallsBackToJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	unknown := map[string]string{"key": "value"}
	if err := renderOutput("table", unknown, &buf); err != nil {
		t.Fatalf("renderOutput unknown type: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "key") {
		t.Errorf("fallback json missing key: %q", out)
	}
}
