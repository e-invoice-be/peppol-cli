package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Example: "  peppol version",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "peppol %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.Date)
		},
	}
}
