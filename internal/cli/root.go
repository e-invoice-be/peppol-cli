package cli

import (
	"fmt"
	"os"

	"github.com/e-invoicebe/peppol-cli/internal/version"
	"github.com/spf13/cobra"
)

// GlobalFlags holds flags shared by all commands.
type GlobalFlags struct {
	JSON    bool
	Quiet   bool
	Verbose bool
	NoColor bool
}

var flags GlobalFlags

// NewRootCmd creates the root peppol command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "peppol",
		Short:   "CLI for the e-invoice.be Peppol Access Point API",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().BoolVar(&flags.Quiet, "quiet", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "Enable verbose output")
	cmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "Disable colored output")

	cmd.AddCommand(newMeCmd())
	cmd.AddCommand(newAuthCmd())

	return cmd
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		if !flags.Quiet {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		// Use exit code from the error if available.
		if exitErr, ok := err.(*ExitError); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Err  error
	Code int
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }
