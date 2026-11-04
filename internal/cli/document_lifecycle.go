package cli

import (
	"errors"
	"fmt"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDocumentLifecycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lifecycle <document-id>",
		Short: "Display lifecycle events stated about a document after creation",
		Long: `Display the lifecycle events stated about an invoice or credit note after
creation: Peppol Message Level Responses and Invoice Responses, and partner
status events, oldest first.

Each event has a route-agnostic canonical status (for example delivered,
rejected, approved, paid) next to the untouched native code of its source.
Native codes without a canonical equivalent show "-" as status.`,
		Example: "  peppol document lifecycle abc123\n  peppol document lifecycle abc123 --json",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentLifecycle,
	}
}

func runDocumentLifecycle(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	lifecycle, err := c.GetDocumentLifecycle(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{
				Err:  fmt.Errorf("document not found. List documents with 'peppol inbox list' or 'peppol outbox list'"),
				Code: 4,
			}
		}
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(lifecycle)
	}

	return renderLifecycle(r, lifecycle)
}

func renderLifecycle(r *output.Renderer, lifecycle *client.DocumentLifecycle) error {
	if len(lifecycle.Events) == 0 {
		fmt.Fprintln(r.Writer(), "No lifecycle events.")
		return nil
	}

	headers := []string{"Occurred", "Status", "Level", "Source", "Dir", "Native Code", "Reason"}
	rows := make([][]string, 0, len(lifecycle.Events))
	for _, ev := range lifecycle.Events {
		status := "-"
		if ev.CanonicalStatus != nil {
			status = string(*ev.CanonicalStatus)
		}
		rows = append(rows, []string{
			ev.OccurredAt.Format("2006-01-02 15:04:05"),
			status,
			string(ev.Level),
			string(ev.Source),
			string(ev.Direction),
			deref(ev.NativeCode, "-"),
			formatLifecycleReason(ev.ReasonCode, ev.Reason),
		})
	}
	return r.Table(headers, rows)
}

// formatLifecycleReason renders a reason as "CODE: text", or whichever part is set.
func formatLifecycleReason(code, reason *string) string {
	hasCode := code != nil && *code != ""
	hasReason := reason != nil && *reason != ""
	switch {
	case hasCode && hasReason:
		return *code + ": " + *reason
	case hasCode:
		return *code
	case hasReason:
		return *reason
	default:
		return "-"
	}
}
