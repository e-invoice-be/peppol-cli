package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

func newDocumentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "document",
		Aliases: []string{"doc"},
		Short:   "Manage documents",
		Example: "  peppol document get <id>\n  peppol doc get <id> --json",
	}

	cmd.AddCommand(newDocumentGetCmd())
	cmd.AddCommand(newDocumentTimelineCmd())
	cmd.AddCommand(newDocumentLifecycleCmd())
	cmd.AddCommand(newAttachmentCmd())
	cmd.AddCommand(newDocumentCreateCmd())
	cmd.AddCommand(newDocumentSendCmd())
	cmd.AddCommand(newDocumentValidateCmd())
	cmd.AddCommand(newDocumentDeleteCmd())
	cmd.AddCommand(newDocumentUBLCmd())

	return cmd
}

func newDocumentGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <document-id>",
		Short:   "Display document details",
		Example: "  peppol document get abc123\n  peppol document get abc123 --full --json",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentGet,
	}

	cmd.Flags().Bool("full", false, "Show full details including line items")

	return cmd
}

func runDocumentGet(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	doc, err := c.GetDocument(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{
				Err:  errors.New("document not found. List documents with 'peppol inbox list' or 'peppol outbox list'"),
				Code: 4,
			}
		}
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
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
	if err := r.KeyValue(documentHeaderPairs(doc)); err != nil {
		return err
	}

	// Parties section
	if doc.CustomerName != nil || doc.VendorName != nil {
		fmt.Fprintln(r.Writer())
		if err := r.KeyValue(documentPartyPairs(doc)); err != nil {
			return err
		}
	}

	// Totals section
	if doc.Subtotal != nil || doc.InvoiceTotal != nil {
		fmt.Fprintln(r.Writer())
		if err := r.KeyValue(documentTotalPairs(doc)); err != nil {
			return err
		}
	}

	// Payment section
	if doc.PaymentTerm != nil || len(doc.PaymentDetails) > 0 {
		fmt.Fprintln(r.Writer())
		if err := r.KeyValue(documentPaymentPairs(doc)); err != nil {
			return err
		}
	}

	// Line Items section (only with --full)
	if full && len(doc.Items) > 0 {
		fmt.Fprintln(r.Writer())
		headers, rows := documentItemsTable(doc.Items)
		if err := r.Table(headers, rows); err != nil {
			return err
		}
	}

	return nil
}

// appendKV appends a key/value pair when value is non-nil.
func appendKV(pairs []output.KVPair, key string, value *string) []output.KVPair {
	if value == nil {
		return pairs
	}
	return append(pairs, output.KVPair{Key: key, Value: *value})
}

func documentHeaderPairs(doc *client.DocumentResponse) []output.KVPair {
	pairs := []output.KVPair{
		{Key: "ID", Value: doc.ID},
		{Key: "Type", Value: string(doc.DocumentType)},
		{Key: "State", Value: output.StatusBadge(string(doc.State))},
		{Key: "Direction", Value: string(doc.Direction)},
	}
	pairs = appendKV(pairs, "Invoice ID", doc.InvoiceID)
	pairs = appendKV(pairs, "Invoice Date", doc.InvoiceDate)
	pairs = appendKV(pairs, "Due Date", doc.DueDate)
	pairs = appendKV(pairs, "Preceding Invoice", doc.PrecedingInvoiceID)
	pairs = appendKV(pairs, "Preceding Invoice Date", doc.PrecedingInvoiceDate)
	pairs = appendKV(pairs, "Buyer Reference", doc.BuyerReference)
	pairs = appendKV(pairs, "Purchase Order", doc.PurchaseOrder)
	pairs = appendKV(pairs, "Sales Order", doc.SalesOrder)
	return append(pairs, output.KVPair{Key: "Created", Value: doc.CreatedAt.Format("2006-01-02")})
}

func documentPartyPairs(doc *client.DocumentResponse) []output.KVPair {
	var pairs []output.KVPair
	pairs = appendKV(pairs, "Customer", doc.CustomerName)
	pairs = appendKV(pairs, "Customer Tax ID", doc.CustomerTaxID)
	pairs = appendKV(pairs, "Customer Phone", doc.CustomerPhone)
	pairs = appendKV(pairs, "Vendor", doc.VendorName)
	pairs = appendKV(pairs, "Vendor Tax ID", doc.VendorTaxID)
	pairs = appendKV(pairs, "Vendor Phone", doc.VendorPhone)
	return pairs
}

func documentTotalPairs(doc *client.DocumentResponse) []output.KVPair {
	cur := doc.Currency
	if cur == "" {
		cur = "EUR"
	}
	amount := func(key string, v *string) []output.KVPair {
		if v == nil {
			return nil
		}
		return []output.KVPair{{Key: key, Value: *v + " " + cur}}
	}
	var pairs []output.KVPair
	pairs = append(pairs, amount("Subtotal", doc.Subtotal)...)
	pairs = append(pairs, amount("VAT", doc.TotalTax)...)
	if doc.TotalTaxInTaxCurrency != nil && doc.TaxCurrency != nil {
		pairs = append(pairs, output.KVPair{Key: "VAT (" + *doc.TaxCurrency + ")", Value: *doc.TotalTaxInTaxCurrency + " " + *doc.TaxCurrency})
	}
	pairs = append(pairs, amount("Total", doc.InvoiceTotal)...)
	pairs = append(pairs, amount("Rounding", doc.RoundingAmount)...)
	pairs = append(pairs, amount("Amount Due", doc.AmountDue)...)
	return pairs
}

func documentPaymentPairs(doc *client.DocumentResponse) []output.KVPair {
	var pairs []output.KVPair
	pairs = appendKV(pairs, "Payment Term", doc.PaymentTerm)
	for _, pd := range doc.PaymentDetails {
		pairs = appendKV(pairs, "IBAN", pd.IBAN)
		pairs = appendKV(pairs, "SWIFT", pd.SWIFT)
		pairs = appendKV(pairs, "Reference", pd.PaymentReference)
	}
	return pairs
}

// documentItemsTable builds the line-item table, adding the optional Name,
// Base Qty and Attributes columns only when at least one item uses them.
func documentItemsTable(items []client.LineItem) (headers []string, rows [][]string) {
	hasName, hasBaseQty, hasAttributes := false, false, false
	for _, item := range items {
		if item.Name != nil && *item.Name != "" {
			hasName = true
		}
		if !isDefaultPriceBaseQuantity(item.PriceBaseQuantity) {
			hasBaseQty = true
		}
		if len(item.ItemAttributes) > 0 {
			hasAttributes = true
		}
	}
	headers = []string{"#"}
	if hasName {
		headers = append(headers, "Name")
	}
	headers = append(headers, "Description", "Qty", "Unit Price")
	if hasBaseQty {
		headers = append(headers, "Base Qty")
	}
	headers = append(headers, "Amount")
	if hasAttributes {
		headers = append(headers, "Attributes")
	}
	for i, item := range items {
		row := []string{strconv.Itoa(i + 1)}
		if hasName {
			row = append(row, deref(item.Name, "-"))
		}
		row = append(row, deref(item.Description, "-"), deref(item.Quantity, "-"), deref(item.UnitPrice, "-"))
		if hasBaseQty {
			row = append(row, deref(item.PriceBaseQuantity, "1"))
		}
		row = append(row, deref(item.Amount, "-"))
		if hasAttributes {
			row = append(row, formatItemAttributes(item.ItemAttributes))
		}
		rows = append(rows, row)
	}
	return headers, rows
}

// isDefaultPriceBaseQuantity reports whether a line item's price base
// quantity (BT-149) is absent or equal to the API default of 1.
func isDefaultPriceBaseQuantity(q *string) bool {
	if q == nil || *q == "" {
		return true
	}
	v, err := strconv.ParseFloat(*q, 64)
	return err == nil && v == 1
}

// formatItemAttributes renders item attributes as "name=value, name".
func formatItemAttributes(attrs []client.ItemAttribute) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		if a.Value != nil {
			parts = append(parts, a.Name+"="+*a.Value)
		} else {
			parts = append(parts, a.Name)
		}
	}
	return strings.Join(parts, ", ")
}

func newDocumentTimelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "timeline <document-id>",
		Short:   "Display document processing timeline",
		Example: "  peppol document timeline abc123",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentTimeline,
	}
}

func runDocumentTimeline(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	timeline, err := c.GetDocumentTimeline(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{
				Err:  errors.New("document not found. List documents with 'peppol inbox list' or 'peppol outbox list'"),
				Code: 4,
			}
		}
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
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
		Use:     "create",
		Short:   "Create a new document",
		Example: "  peppol document create json invoice.json\n  peppol document create ubl invoice.xml\n  peppol document create pdf invoice.pdf",
	}
	cmd.AddCommand(newDocumentCreateJSONCmd())
	cmd.AddCommand(newDocumentCreateUBLCmd())
	cmd.AddCommand(newDocumentCreatePDFCmd())
	return cmd
}

func newDocumentCreateJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "json <file>",
		Short:   "Create a document from a JSON file",
		Example: "  peppol document create json invoice.json\n  peppol document create json invoice.json --construct-pdf",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentCreateJSON,
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
	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
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
		Use:     "ubl <file>",
		Short:   "Create a document from a UBL/XML file",
		Example: "  peppol document create ubl invoice.xml",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentCreateUBL,
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

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
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
		Use:     "pdf <file>",
		Short:   "Create a document from a PDF file",
		Example: "  peppol document create pdf invoice.pdf",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentCreatePDF,
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

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	doc, err := c.CreateDocumentFromPDF(filePath, vendorTaxID, customerTaxID)
	if err != nil {
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(doc)
	}

	return renderPDFCreateResult(r, doc)
}

// renderPDFCreateResult renders the result of a PDF conversion. A failed
// conversion still returns a document.
func renderPDFCreateResult(r *output.Renderer, doc *client.DocumentCreateFromPdfResponse) error {
	if doc.Success {
		r.Success("Document created from PDF successfully.")
	} else {
		r.Error("PDF conversion failed. The document may require manual review.")
		var errPairs []output.KVPair
		if doc.ErrorType != nil {
			errPairs = append(errPairs, output.KVPair{Key: "Error Type", Value: *doc.ErrorType})
		}
		if doc.ErrorMessage != nil {
			errPairs = append(errPairs, output.KVPair{Key: "Error Message", Value: *doc.ErrorMessage})
		}
		if err := r.KeyValue(errPairs); err != nil {
			return err
		}
	}
	fmt.Fprintln(r.Writer())
	return renderDocumentSections(r, &doc.DocumentResponse, false)
}

// --- Send command ---

func newDocumentSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "send <document-id>",
		Short:   "Send a document via Peppol",
		Example: "  peppol document send abc123",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentSend,
	}
	cmd.Flags().String("sender-peppol-id", "", "Override sender Peppol ID")
	cmd.Flags().String("sender-peppol-scheme", "", "Override sender Peppol scheme")
	cmd.Flags().String("receiver-peppol-id", "", "Override receiver Peppol ID")
	cmd.Flags().String("receiver-peppol-scheme", "", "Override receiver Peppol scheme")
	cmd.Flags().String("email", "", "Send notification email")
	_ = cmd.Flags().MarkDeprecated("email", "the API has deprecated this parameter and may remove it")
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

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	doc, err := c.SendDocument(args[0], opts)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: errors.New("document not found"), Code: 4}
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
		Use:     "validate <document-id>",
		Short:   "Validate a document against Peppol BIS Billing 3.0",
		Example: "  peppol document validate abc123",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentValidate,
	}
}

func runDocumentValidate(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	val, err := c.ValidateDocument(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: errors.New("document not found"), Code: 4}
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
		return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
	}
	return &ExitError{Err: err, Code: 1}
}

// --- Delete command ---

func newDocumentDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <document-id>",
		Short:   "Delete a draft document",
		Example: "  peppol document delete abc123\n  peppol document delete abc123 --yes",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentDelete,
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func runDocumentDelete(cmd *cobra.Command, args []string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && !confirm(cmd, fmt.Sprintf("Delete document %s?", args[0])) {
		return nil
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.DeleteDocument(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: errors.New("document not found"), Code: 4}
		}
		return handleDocumentError(err)
	}

	return renderDeleteResult(cmd, result, result.IsDeleted, "Document")
}

// renderDeleteResult reports the outcome of a delete. A 2xx answer with
// is_deleted: false is an error (exit 1), so the user and scripts can tell
// that nothing was deleted. With --json, stdout is still the API's answer;
// the error goes to stderr.
// noun is capitalised ("Document"), as it starts the success message.
func renderDeleteResult(cmd *cobra.Command, result any, isDeleted bool, noun string) error {
	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		if err := r.JSON(result); err != nil {
			return err
		}
	}
	if !isDeleted {
		return &ExitError{Err: errors.New(strings.ToLower(noun) + " was not deleted"), Code: 1}
	}
	if !r.IsJSON() {
		r.Success(noun + " deleted.")
	}
	return nil
}

// --- UBL download command ---

func newDocumentUBLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ubl <document-id>",
		Short:   "Download the UBL XML representation of a document",
		Example: "  peppol document ubl abc123\n  peppol document ubl abc123 -o invoice.xml",
		Args:    cobra.ExactArgs(1),
		RunE:    runDocumentUBL,
	}
	cmd.Flags().StringP("output", "o", "", "Write XML to file instead of stdout")
	return cmd
}

func runDocumentUBL(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	ubl, err := c.GetDocumentUBL(args[0])
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return &ExitError{Err: errors.New("document not found"), Code: 4}
		}
		return handleDocumentError(err)
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(ubl)
	}

	if ubl.SignedURL == nil {
		return &ExitError{Err: errors.New("no UBL file available for this document"), Code: 1}
	}

	// Stream the UBL XML from the signed URL
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, *ubl.SignedURL, http.NoBody)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("downloading UBL: %w", err), Code: 1}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("downloading UBL: %w", err), Code: 1}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ExitError{Err: fmt.Errorf("downloading UBL: HTTP %d", resp.StatusCode), Code: 1}
	}

	outputPath, _ := cmd.Flags().GetString("output")
	if outputPath != "" {
		f, err := os.Create(outputPath) //nolint:gosec // G304: --output path is supplied by the user on purpose
		if err != nil {
			return &ExitError{Err: fmt.Errorf("creating output file: %w", err), Code: 1}
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			_ = f.Close()
			return &ExitError{Err: fmt.Errorf("writing output file: %w", err), Code: 1}
		}
		if err := f.Close(); err != nil {
			return &ExitError{Err: fmt.Errorf("closing output file: %w", err), Code: 1}
		}
		r.Success("UBL XML written to " + outputPath)
		return nil
	}

	_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("writing output: %w", err), Code: 1}
	}
	return nil
}
