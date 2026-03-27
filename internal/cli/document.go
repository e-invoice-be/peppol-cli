package cli

import (
	"errors"
	"fmt"
	"os"
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
	cmd.AddCommand(newAttachmentCmd())
	cmd.AddCommand(newDocumentCreateCmd())
	cmd.AddCommand(newDocumentSendCmd())
	cmd.AddCommand(newDocumentValidateCmd())

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

// --- Create commands ---

func newDocumentCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new document",
	}
	cmd.AddCommand(newDocumentCreateJSONCmd())
	cmd.AddCommand(newDocumentCreateUBLCmd())
	cmd.AddCommand(newDocumentCreatePDFCmd())
	return cmd
}

func newDocumentCreateJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "json <file>",
		Short: "Create a document from a JSON file",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentCreateJSON,
	}
	cmd.Flags().Bool("construct-pdf", false, "Generate a PDF from the document")
	return cmd
}

func runDocumentCreateJSON(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	if _, err := os.Stat(filePath); err != nil {
		return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	constructPDF, _ := cmd.Flags().GetBool("construct-pdf")
	c := client.NewClient(apiKey)
	doc, err := c.CreateDocumentJSON(filePath, constructPDF)
	if err != nil {
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(doc)
	}

	r.Success("Document created successfully.")
	fmt.Fprintln(r.Writer())
	return renderDocumentSections(r, doc, false)
}

func newDocumentCreateUBLCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ubl <file>",
		Short: "Create a document from a UBL/XML file",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentCreateUBL,
	}
}

func runDocumentCreateUBL(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	if _, err := os.Stat(filePath); err != nil {
		return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	doc, err := c.CreateDocumentFromUBL(filePath)
	if err != nil {
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(doc)
	}

	r.Success("Document created from UBL successfully.")
	fmt.Fprintln(r.Writer())
	return renderDocumentSections(r, doc, false)
}

func newDocumentCreatePDFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pdf <file>",
		Short: "Create a document from a PDF file",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentCreatePDF,
	}
	cmd.Flags().String("vendor-tax-id", "", "Vendor tax ID (e.g. BE1018265814)")
	cmd.Flags().String("customer-tax-id", "", "Customer tax ID (e.g. BE1018265814)")
	return cmd
}

func runDocumentCreatePDF(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	if _, err := os.Stat(filePath); err != nil {
		return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	vendorTaxID, _ := cmd.Flags().GetString("vendor-tax-id")
	customerTaxID, _ := cmd.Flags().GetString("customer-tax-id")

	c := client.NewClient(apiKey)
	doc, err := c.CreateDocumentFromPDF(filePath, vendorTaxID, customerTaxID)
	if err != nil {
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(doc)
	}

	if doc.Success {
		r.Success("Document created from PDF successfully.")
	} else {
		r.Error("Document created but may require manual review.")
	}
	fmt.Fprintln(r.Writer())
	return renderDocumentSections(r, &doc.DocumentResponse, false)
}

// --- Send command ---

func newDocumentSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send <document-id>",
		Short: "Send a document via Peppol",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentSend,
	}
	cmd.Flags().String("sender-peppol-id", "", "Override sender Peppol ID")
	cmd.Flags().String("sender-peppol-scheme", "", "Override sender Peppol scheme")
	cmd.Flags().String("receiver-peppol-id", "", "Override receiver Peppol ID")
	cmd.Flags().String("receiver-peppol-scheme", "", "Override receiver Peppol scheme")
	cmd.Flags().String("email", "", "Send notification email")
	return cmd
}

func runDocumentSend(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	opts := client.SendDocumentOptions{}
	opts.SenderPeppolID, _ = cmd.Flags().GetString("sender-peppol-id")
	opts.SenderPeppolScheme, _ = cmd.Flags().GetString("sender-peppol-scheme")
	opts.ReceiverPeppolID, _ = cmd.Flags().GetString("receiver-peppol-id")
	opts.ReceiverPeppolScheme, _ = cmd.Flags().GetString("receiver-peppol-scheme")
	opts.Email, _ = cmd.Flags().GetString("email")

	c := client.NewClient(apiKey)
	doc, err := c.SendDocument(args[0], opts)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: fmt.Errorf("document not found"), Code: 4}
		}
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(doc)
	}

	r.Success("Document sent successfully.")
	fmt.Fprintln(r.Writer())
	return renderDocumentSections(r, doc, false)
}

// --- Validate command ---

func newDocumentValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <document-id>",
		Short: "Validate a document against Peppol BIS Billing 3.0",
		Args:  cobra.ExactArgs(1),
		RunE:  runDocumentValidate,
	}
}

func runDocumentValidate(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	val, err := c.ValidateDocument(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: fmt.Errorf("document not found"), Code: 4}
		}
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(val)
	}

	return renderValidation(r, val)
}

func renderValidation(r *output.Renderer, val *client.ValidationResponse) error {
	if val.IsValid {
		r.Success("Document is valid.")
	} else {
		r.Error("Document is not valid.")
	}

	if len(val.Issues) == 0 {
		return nil
	}

	fmt.Fprintln(r.Writer())
	headers := []string{"Type", "Message", "Rule"}
	var rows [][]string
	for _, issue := range val.Issues {
		rule := deref(issue.RuleID, "-")
		rows = append(rows, []string{string(issue.Type), issue.Message, rule})
	}
	return r.Table(headers, rows)
}

// handleDocumentError converts client errors to ExitErrors.
func handleDocumentError(err error) error {
	if errors.Is(err, client.ErrUnauthorized) {
		return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
	}
	return &ExitError{Err: err, Code: 1}
}
