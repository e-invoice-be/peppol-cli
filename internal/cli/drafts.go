package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/spf13/cobra"
)

func newDraftsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drafts",
		Short: "Browse draft documents",
	}

	cmd.AddCommand(newDraftsListCmd())

	return cmd
}

// --- drafts list ---

var draftsListFlags DocumentListFlags

func newDraftsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all draft documents",
		Args:  cobra.NoArgs,
		RunE:  runDraftsList,
	}
	draftsListFlags.BindCommonFlags(cmd)
	draftsListFlags.BindStateFlag(cmd)
	return cmd
}

func runDraftsList(cmd *cobra.Command, args []string) error {
	if err := draftsListFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListDrafts(draftsListFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartyBuyer)
}
