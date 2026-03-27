package cli

import (
	"fmt"
	"os"

	"github.com/e-invoicebe/peppol-cli/internal/output"
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			r := output.NewRenderer(cmd.OutOrStdout(), flags.JSON, flags.Quiet, flags.NoColor)
			cmd.SetContext(output.WithRenderer(cmd.Context(), r))
			return nil
		},
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
		code := 1
		if exitErr, ok := err.(*ExitError); ok {
			code = exitErr.Code
		}
		if flags.JSON {
			r := output.NewRenderer(os.Stderr, true, flags.Quiet, flags.NoColor)
			_ = r.JSONError(err, code)
		} else if !flags.Quiet {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(code)
	}
}

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Err  error
	Code int
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }
