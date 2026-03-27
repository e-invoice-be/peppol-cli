package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/spf13/cobra"
)

func newOutboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outbox",
		Short: "Browse sent documents",
	}

	cmd.AddCommand(newOutboxListCmd())
	cmd.AddCommand(newOutboxDraftsCmd())

	return cmd
}

// --- outbox list ---

var outboxListFlags DocumentListFlags

func newOutboxListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sent documents",
		Args:  cobra.NoArgs,
		RunE:  runOutboxList,
	}
	outboxListFlags.BindCommonFlags(cmd)
	outboxListFlags.BindReceiverFlag(cmd)
	return cmd
}

func runOutboxList(cmd *cobra.Command, args []string) error {
	if err := outboxListFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListOutbox(outboxListFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartyBuyer)
}

// --- outbox drafts ---

var outboxDraftsFlags DocumentListFlags

func newOutboxDraftsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drafts",
		Short: "List outbox draft documents",
		Args:  cobra.NoArgs,
		RunE:  runOutboxDrafts,
	}
	outboxDraftsFlags.BindCommonFlags(cmd)
	outboxDraftsFlags.BindReceiverFlag(cmd)
	return cmd
}

func runOutboxDrafts(cmd *cobra.Command, args []string) error {
	if err := outboxDraftsFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListOutboxDrafts(outboxDraftsFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartyBuyer)
}
