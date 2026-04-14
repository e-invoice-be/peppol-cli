package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/e-invoicebe/peppol-cli/internal/version"
	"github.com/spf13/cobra"
)

// GlobalFlags holds flags shared by all commands.
type GlobalFlags struct {
	JSON      bool
	Quiet     bool
	Verbose   bool
	NoColor   bool
	Workspace string
}

var flags GlobalFlags

// NewRootCmd creates the root peppol command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "peppol",
		Short:         "CLI for the e-invoice.be Peppol Access Point API",
		Version:       fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			r := output.NewRenderer(cmd.OutOrStdout(), flags.JSON, flags.Quiet, flags.NoColor)
			cmd.SetContext(output.WithRenderer(cmd.Context(), r))
			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(&flags.JSON, "json", "j", false, "Output as JSON")
	cmd.PersistentFlags().BoolVarP(&flags.Quiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().StringVarP(&flags.Workspace, "workspace", "w", "", "Override active workspace for this command")

	cmd.AddCommand(newMeCmd())
	cmd.AddCommand(newWorkspaceCmd())
	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newStatsCmd())
	cmd.AddCommand(newDocumentCmd())
	cmd.AddCommand(newInboxCmd())
	cmd.AddCommand(newOutboxCmd())
	cmd.AddCommand(newDraftsCmd())
	cmd.AddCommand(newLookupCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := NewRootCmd()
	cmd.SetContext(ctx)
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

// clientOpts returns common client options based on global flags.
func clientOpts() []client.ClientOption {
	var opts []client.ClientOption
	if flags.Verbose {
		opts = append(opts, client.WithVerbose(os.Stderr))
	}
	return opts
}

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Err  error
	Code int
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }
