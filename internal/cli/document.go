package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDocumentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "document",
		Aliases: []string{"doc"},
		Short:   "Manage documents",
	}

	cmd.AddCommand(newDocumentGetCmd())
	cmd.AddCommand(newDocumentTimelineCmd())

	return cmd
}

func newDocumentGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <document-id>",
		Short: "Display document details",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentGet,
	}

	cmd.Flags().Bool("full", false, "Show full details including line items")

	return cmd
}

func runDocumentGet(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	doc, err := c.GetDocument(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{
				Err:  fmt.Errorf("document not found. List documents with 'peppol inbox list' or 'peppol outbox list'"),
				Code: 4,
			}
		}
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(doc)
	}

	full, _ := cmd.Flags().GetBool("full")
	return renderDocumentSections(r, doc, full)
}

func renderDocumentSections(r *output.Renderer, doc *client.DocumentResponse, full bool) error {
	// Header section
	pairs := []output.KVPair{
		{Key: "ID", Value: doc.ID},
		{Key: "Type", Value: string(doc.DocumentType)},
		{Key: "State", Value: output.StatusBadge(string(doc.State))},
		{Key: "Direction", Value: string(doc.Direction)},
	}
	if doc.InvoiceID != nil {
		pairs = append(pairs, output.KVPair{Key: "Invoice ID", Value: *doc.InvoiceID})
	}
	if doc.InvoiceDate != nil {
		pairs = append(pairs, output.KVPair{Key: "Invoice Date", Value: *doc.InvoiceDate})
	}
	if doc.DueDate != nil {
		pairs = append(pairs, output.KVPair{Key: "Due Date", Value: *doc.DueDate})
	}
	pairs = append(pairs, output.KVPair{Key: "Created", Value: doc.CreatedAt.Format("2006-01-02")})
	if err := r.KeyValue(pairs); err != nil {
		return err
	}

	// Parties section
	if doc.CustomerName != nil || doc.VendorName != nil {
		fmt.Fprintln(r.Writer())
		var partyPairs []output.KVPair
		if doc.CustomerName != nil {
			partyPairs = append(partyPairs, output.KVPair{Key: "Customer", Value: *doc.CustomerName})
		}
		if doc.CustomerTaxID != nil {
			partyPairs = append(partyPairs, output.KVPair{Key: "Customer Tax ID", Value: *doc.CustomerTaxID})
		}
		if doc.VendorName != nil {
			partyPairs = append(partyPairs, output.KVPair{Key: "Vendor", Value: *doc.VendorName})
		}
		if doc.VendorTaxID != nil {
			partyPairs = append(partyPairs, output.KVPair{Key: "Vendor Tax ID", Value: *doc.VendorTaxID})
		}
		if err := r.KeyValue(partyPairs); err != nil {
			return err
		}
	}

	// Totals section
	if doc.Subtotal != nil || doc.InvoiceTotal != nil {
		fmt.Fprintln(r.Writer())
		cur := doc.Currency
		if cur == "" {
			cur = "EUR"
		}
		var totalPairs []output.KVPair
		if doc.Subtotal != nil {
			totalPairs = append(totalPairs, output.KVPair{Key: "Subtotal", Value: *doc.Subtotal + " " + cur})
		}
		if doc.TotalTax != nil {
			totalPairs = append(totalPairs, output.KVPair{Key: "VAT", Value: *doc.TotalTax + " " + cur})
		}
		if doc.InvoiceTotal != nil {
			totalPairs = append(totalPairs, output.KVPair{Key: "Total", Value: *doc.InvoiceTotal + " " + cur})
		}
		if doc.AmountDue != nil {
			totalPairs = append(totalPairs, output.KVPair{Key: "Amount Due", Value: *doc.AmountDue + " " + cur})
		}
		if err := r.KeyValue(totalPairs); err != nil {
			return err
		}
	}

	// Payment section
	if doc.PaymentTerm != nil || len(doc.PaymentDetails) > 0 {
		fmt.Fprintln(r.Writer())
		var payPairs []output.KVPair
		if doc.PaymentTerm != nil {
			payPairs = append(payPairs, output.KVPair{Key: "Payment Term", Value: *doc.PaymentTerm})
		}
		for _, pd := range doc.PaymentDetails {
			if pd.IBAN != nil {
				payPairs = append(payPairs, output.KVPair{Key: "IBAN", Value: *pd.IBAN})
			}
			if pd.SWIFT != nil {
				payPairs = append(payPairs, output.KVPair{Key: "SWIFT", Value: *pd.SWIFT})
			}
			if pd.PaymentReference != nil {
				payPairs = append(payPairs, output.KVPair{Key: "Reference", Value: *pd.PaymentReference})
			}
		}
		if err := r.KeyValue(payPairs); err != nil {
			return err
		}
	}

	// Line Items section (only with --full)
	if full && len(doc.Items) > 0 {
		fmt.Fprintln(r.Writer())
		headers := []string{"#", "Description", "Qty", "Unit Price", "Amount"}
		var rows [][]string
		for i, item := range doc.Items {
			desc := deref(item.Description, "-")
			qty := deref(item.Quantity, "-")
			price := deref(item.UnitPrice, "-")
			amount := deref(item.Amount, "-")
			rows = append(rows, []string{fmt.Sprintf("%d", i+1), desc, qty, price, amount})
		}
		if err := r.Table(headers, rows); err != nil {
			return err
		}
	}

	return nil
}

func newDocumentTimelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "timeline <document-id>",
		Short: "Display document processing timeline",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentTimeline,
	}
}

func runDocumentTimeline(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	timeline, err := c.GetDocumentTimeline(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{
				Err:  fmt.Errorf("document not found. List documents with 'peppol inbox list' or 'peppol outbox list'"),
				Code: 4,
			}
		}
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(timeline)
	}

	return renderTimeline(r, timeline)
}

func renderTimeline(r *output.Renderer, timeline *client.DocumentTimeline) error {
	if len(timeline.Events) == 0 {
		fmt.Fprintln(r.Writer(), "No timeline events.")
		return nil
	}

	for _, event := range timeline.Events {
		ts := event.Timestamp.Format("2006-01-02 15:04:05")
		name := formatEventType(string(event.EventType))
		fmt.Fprintf(r.Writer(), "%s  %s\n", ts, name)
	}
	return nil
}

func deref(s *string, fallback string) string {
	if s != nil {
		return *s
	}
	return fallback
}

func formatEventType(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
