package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nowi5/kleido/internal/client"
	"github.com/spf13/cobra"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "User management commands",
	}
	cmd.AddCommand(
		newUsersMeCmd(),
		newUsersGetCmd(),
		newUsersListCmd(),
		newUsersUpdateCmd(),
		newUsersDeleteCmd(),
	)
	return cmd
}

func newUsersMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the authenticated user's profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := requireAuth()
			c := newClient(cfg)
			user, err := c.Users.Me(context.Background())
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return renderOutput(outputFormat, user, cmd.OutOrStdout())
		},
	}
}

func newUsersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a user by UUID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := requireAuth()
			c := newClient(cfg)
			user, err := c.Users.Get(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return renderOutput(outputFormat, user, cmd.OutOrStdout())
		},
	}
}

func newUsersListCmd() *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all users (admin only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := requireAuth()
			c := newClient(cfg)
			resp, err := c.Users.List(context.Background(), page, perPage)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return renderOutput(outputFormat, resp, cmd.OutOrStdout())
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based)")
	cmd.Flags().IntVar(&perPage, "per-page", 20, "Items per page (max 100)")
	return cmd
}

func newUsersUpdateCmd() *cobra.Command {
	var email, role string
	var active string // "true"/"false"/""
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a user's fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := requireAuth()
			c := newClient(cfg)
			req := client.UpdateUserRequest{}
			if cmd.Flags().Changed("email") {
				req.Email = &email
			}
			if cmd.Flags().Changed("role") {
				req.Role = &role
			}
			if cmd.Flags().Changed("active") {
				isActive := active == "true"
				req.IsActive = &isActive
			}
			user, err := c.Users.Update(context.Background(), args[0], req)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return renderOutput(outputFormat, user, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "New email address")
	cmd.Flags().StringVar(&role, "role", "", "New role (user|admin)")
	cmd.Flags().StringVar(&active, "active", "", "Set active status (true|false)")
	return cmd
}

func newUsersDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete (deactivate) a user (admin only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if !confirmPrompt(cmd, fmt.Sprintf("Delete user %s?", args[0])) {
					//nolint:errcheck // prompt output to stdout is intentionally not checked
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			cfg := requireAuth()
			c := newClient(cfg)
			if err := c.Users.Delete(context.Background(), args[0]); err != nil {
				return fmt.Errorf("%w", err)
			}
			//nolint:errcheck // prompt output to stdout is intentionally not checked
			fmt.Fprintln(cmd.OutOrStdout(), "User deleted.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

// confirmPrompt asks the user to confirm an action by typing "y" or "yes".
// Returns true if confirmed.
func confirmPrompt(cmd *cobra.Command, message string) bool {
	//nolint:errcheck // prompt output to stdout is intentionally not checked
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", message)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}
