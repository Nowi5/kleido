// Package cli implements the kleido command-line interface using cobra.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/nowi5/kleido/internal/client"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

// renderOutput writes data to w in the chosen format.
// Supported formats: "table" (default), "json", "yaml".
func renderOutput(format string, data any, w io.Writer) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		return nil
	case "yaml":
		enc := yaml.NewEncoder(w)
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("render yaml: %w", err)
		}
		return nil
	default:
		return renderTable(data, w)
	}
}

// renderTable dispatches to the appropriate table renderer based on type.
// Falls back to JSON for unknown types.
func renderTable(data any, w io.Writer) error {
	switch v := data.(type) {
	case *client.UserResponse:
		renderUserTable(v, w)
	case *client.ListUsersResponse:
		renderUsersTable(v.Data, w)
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("render table fallback json: %w", err)
		}
	}
	return nil
}

// renderUserTable prints a single user as a two-column key/value table.
func renderUserTable(u *client.UserResponse, w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"FIELD", "VALUE"})
	table.SetBorder(false)
	table.Append([]string{"ID", u.ID})
	table.Append([]string{"EMAIL", u.Email})
	table.Append([]string{"ROLE", u.Role})
	table.Append([]string{"ACTIVE", fmt.Sprintf("%v", u.IsActive)})
	table.Append([]string{"CREATED", u.CreatedAt.Format(time.RFC3339)})
	table.Render()
}

// renderUsersTable prints a slice of users as a multi-column table.
func renderUsersTable(users []*client.UserResponse, w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"ID", "EMAIL", "ROLE", "ACTIVE", "CREATED"})
	table.SetBorder(false)
	for _, u := range users {
		table.Append([]string{
			u.ID,
			u.Email,
			u.Role,
			fmt.Sprintf("%v", u.IsActive),
			u.CreatedAt.Format(time.RFC3339),
		})
	}
	table.Render()
}
