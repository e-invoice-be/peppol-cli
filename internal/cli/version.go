package cli

import (
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "peppol %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.Date)
		},
	}
}
