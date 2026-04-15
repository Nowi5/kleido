package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if short {
				//nolint:errcheck // version output to stdout is intentionally not checked
				fmt.Fprintln(cmd.OutOrStdout(), appVersion)
			} else {
				//nolint:errcheck // version output to stdout is intentionally not checked
				fmt.Fprintf(cmd.OutOrStdout(), "kleido version %s\n", appVersion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print only the version string")
	return cmd
}
