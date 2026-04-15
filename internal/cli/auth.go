package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/nowi5/kleido/internal/client"
	"github.com/nowi5/kleido/pkg/configstore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and store credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			//nolint:errcheck // prompt output to stdout is intentionally not checked
			fmt.Fprint(cmd.OutOrStdout(), "Email: ")
			var email string
			if _, err := fmt.Fscanln(cmd.InOrStdin(), &email); err != nil {
				return fmt.Errorf("read email: %w", err)
			}

			password, err := readPassword(cmd)
			if err != nil {
				return err
			}

			c := client.New(apiURL, "")
			resp, err := c.Auth.Login(context.Background(), client.LoginRequest{
				Email:    email,
				Password: password,
			})
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			cfg := &configstore.Config{
				APIURL:      apiURL,
				AccessToken: resp.AccessToken,
				ExpiresAt:   resp.ExpiresAt,
			}
			if err := configstore.Save(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			//nolint:errcheck // prompt output to stdout is intentionally not checked
			fmt.Fprintln(cmd.OutOrStdout(), "Logged in successfully.")
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API base URL")
	return cmd
}

// readPassword reads a password from MYAPP_PASSWORD env var or from stdin with echo disabled.
// The password is never logged or stored in the config — only the token is persisted.
func readPassword(cmd *cobra.Command) (string, error) {
	if pw := os.Getenv("MYAPP_PASSWORD"); pw != "" {
		return pw, nil
	}
	//nolint:errcheck // prompt output to stdout is intentionally not checked
	fmt.Fprint(cmd.OutOrStdout(), "Password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // stdin fd is safe
	//nolint:errcheck // newline after masked input; write error is unrecoverable
	fmt.Fprintln(cmd.OutOrStdout())
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke session and clear stored credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := checkAuth()
			if err != nil {
				// If not logged in, clearing the config is still a success.
				//nolint:errcheck // prompt output to stdout is intentionally not checked
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				return nil
			}

			c := newClient(cfg)
			if err := c.Auth.Logout(context.Background()); err != nil {
				// Log the error but proceed to clear local config.
				//nolint:errcheck // warning to stderr; write error is unrecoverable
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: server logout failed: %v\n", err)
			}

			if err := configstore.ClearToken(); err != nil {
				return fmt.Errorf("clear token: %w", err)
			}

			//nolint:errcheck // prompt output to stdout is intentionally not checked
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}
