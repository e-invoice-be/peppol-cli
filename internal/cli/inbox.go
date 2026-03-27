package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/spf13/cobra"
)

func newInboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Browse received documents",
	}

	cmd.AddCommand(newInboxListCmd())
	cmd.AddCommand(newInboxInvoicesCmd())
	cmd.AddCommand(newInboxCreditNotesCmd())

	return cmd
}

// --- inbox list ---

var inboxListFlags DocumentListFlags

func newInboxListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List received documents",
		Args:  cobra.NoArgs,
		RunE:  runInboxList,
	}
	inboxListFlags.BindCommonFlags(cmd)
	inboxListFlags.BindSenderFlag(cmd)
	return cmd
}

func runInboxList(cmd *cobra.Command, args []string) error {
	if err := inboxListFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListInbox(inboxListFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartySeller)
}

// --- inbox invoices ---

var inboxInvoicesFlags DocumentListFlags

func newInboxInvoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "List received invoices",
		Args:  cobra.NoArgs,
		RunE:  runInboxInvoices,
	}
	inboxInvoicesFlags.BindCommonFlags(cmd)
	inboxInvoicesFlags.BindSenderFlag(cmd)
	return cmd
}

func runInboxInvoices(cmd *cobra.Command, args []string) error {
	if err := inboxInvoicesFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListInboxInvoices(inboxInvoicesFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartySeller)
}

// --- inbox credit-notes ---

var inboxCreditNotesFlags DocumentListFlags

func newInboxCreditNotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credit-notes",
		Short: "List received credit notes",
		Args:  cobra.NoArgs,
		RunE:  runInboxCreditNotes,
	}
	inboxCreditNotesFlags.BindCommonFlags(cmd)
	inboxCreditNotesFlags.BindSenderFlag(cmd)
	return cmd
}

func runInboxCreditNotes(cmd *cobra.Command, args []string) error {
	if err := inboxCreditNotesFlags.Validate(); err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	result, err := c.ListInboxCreditNotes(inboxCreditNotesFlags.ToParams())
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	return renderDocumentList(cmd, result, CounterpartySeller)
}
