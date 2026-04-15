package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/nowi5/kleido/internal/client"
	"github.com/nowi5/kleido/pkg/configstore"
	"github.com/spf13/cobra"
)

var appVersion = "dev"

// SetVersion injects the build-time version string into the CLI.
func SetVersion(v string) { appVersion = v }

// outputFormat is the value of the global --output flag.
var outputFormat string

// NewRootCmd creates and returns the fully-wired root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kleido",
		Short:         "kleido — CLI for the kleido API",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table",
		"Output format: table, json, yaml")
	root.AddCommand(
		newAuthCmd(),
		newUsersCmd(),
		newVersionCmd(),
		newCompletionCmd(root),
	)
	return root
}

// checkAuth loads the stored config and validates the access token.
// Returns an error (not os.Exit) so it can be unit-tested.
func checkAuth() (*configstore.Config, error) {
	cfg, err := configstore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("not logged in — run: kleido auth login")
	}
	// A zero ExpiresAt means the token never expires (e.g. test tokens).
	if !cfg.ExpiresAt.IsZero() && time.Now().After(cfg.ExpiresAt) {
		return nil, fmt.Errorf("access token has expired — run: kleido auth login")
	}
	return cfg, nil
}

// requireAuth calls checkAuth and exits 1 on failure.
// Use checkAuth directly in unit tests.
func requireAuth() *configstore.Config {
	cfg, err := checkAuth()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return cfg
}

// newClient constructs a typed API client from the persisted config.
func newClient(cfg *configstore.Config) *client.Client {
	return client.New(cfg.APIURL, cfg.AccessToken)
}
