package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

var validMailboxStatuses = []string{"pending", "success", "failed"}
var validMailboxSortFields = []string{"received_at", "created_at"}

func newMailboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mailbox",
		Short:   "Browse inbound emails and their attachments",
		Example: "  peppol mailbox list --status failed\n  peppol mailbox get <email-id> --json",
	}

	cmd.AddCommand(newMailboxListCmd())
	cmd.AddCommand(newMailboxGetCmd())
	cmd.AddCommand(newMailboxAttachmentCmd())
	cmd.AddCommand(newMailboxReprocessCmd())

	return cmd
}

// --- mailbox list ---

func newMailboxListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List inbound emails",
		Example: "  peppol mailbox list\n  peppol mailbox list --status failed --from 2026-01-01T00:00:00Z",
		Args:    cobra.NoArgs,
		RunE:    runMailboxList,
	}
	cmd.Flags().String("status", "", "Filter by status ("+strings.Join(validMailboxStatuses, ", ")+"); takes precedence over --processed")
	cmd.Flags().Bool("processed", false, "Filter by processed state (use --processed=false for unprocessed emails)")
	cmd.Flags().String("from", "", "Received on or after this date-time (RFC 3339, e.g. 2026-01-01T00:00:00Z)")
	cmd.Flags().String("to", "", "Received on or before this date-time (RFC 3339, e.g. 2026-01-31T23:59:59Z)")
	cmd.Flags().String("search", "", "Search sender email, subject, message ID and attachment filenames")
	cmd.Flags().String("sort-by", "", "Sort field ("+strings.Join(validMailboxSortFields, ", ")+")")
	cmd.Flags().String("sort-order", "", "Sort direction (asc, desc)")
	cmd.Flags().Int("page", 1, "Page number")
	cmd.Flags().Int("page-size", 20, "Results per page (max 100)")
	return cmd
}

func mailboxListParams(cmd *cobra.Command) (client.MailboxListParams, error) {
	var p client.MailboxListParams
	p.Status, _ = cmd.Flags().GetString("status")
	p.ReceivedFrom, _ = cmd.Flags().GetString("from")
	p.ReceivedTo, _ = cmd.Flags().GetString("to")
	p.Search, _ = cmd.Flags().GetString("search")
	p.SortBy, _ = cmd.Flags().GetString("sort-by")
	p.SortOrder, _ = cmd.Flags().GetString("sort-order")
	p.Page, _ = cmd.Flags().GetInt("page")
	p.PageSize, _ = cmd.Flags().GetInt("page-size")

	if cmd.Flags().Changed("processed") {
		processed, _ := cmd.Flags().GetBool("processed")
		p.Processed = &processed
	}

	if p.Status != "" && !slices.Contains(validMailboxStatuses, p.Status) {
		return p, fmt.Errorf("invalid --status %q, must be one of: %s", p.Status, strings.Join(validMailboxStatuses, ", "))
	}
	if p.SortBy != "" && !slices.Contains(validMailboxSortFields, p.SortBy) {
		return p, fmt.Errorf("invalid --sort-by %q, must be one of: %s", p.SortBy, strings.Join(validMailboxSortFields, ", "))
	}
	if p.SortOrder != "" && !slices.Contains(validSortOrders, p.SortOrder) {
		return p, fmt.Errorf("invalid --sort-order %q, must be one of: %s", p.SortOrder, strings.Join(validSortOrders, ", "))
	}
	return p, nil
}

func runMailboxList(cmd *cobra.Command, args []string) error {
	params, err := mailboxListParams(cmd)
	if err != nil {
		return err
	}
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.ListMailbox(params)
	if err != nil {
		return handleMailboxError(err, "inbound email")
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(result)
	}

	return renderMailboxList(r, result)
}

func renderMailboxList(r *output.Renderer, result *client.PaginatedInboundEmails) error {
	if len(result.Items) == 0 {
		fmt.Fprintln(r.Writer(), "No inbound emails found.")
		return nil
	}

	headers := []string{"ID", "PROCESSED", "FROM", "SUBJECT", "ATTACHMENTS", "RECEIVED"}
	rows := make([][]string, 0, len(result.Items))
	for _, mail := range result.Items {
		rows = append(rows, []string{
			mail.ID,
			yesNo(mail.Processed),
			mail.SenderEmail,
			deref(mail.Subject, ""),
			strconv.Itoa(mail.AttachmentCount),
			mailboxReceived(&mail).Format("2006-01-02 15:04"),
		})
	}

	if err := r.Table(headers, rows); err != nil {
		return err
	}
	r.Pagination("emails", result.Page, result.PageSize, result.Total)
	return nil
}

func renderMailboxEmail(r *output.Renderer, mail *client.InboundEmailResponse) error {
	from := mail.SenderEmail
	if mail.SenderName != nil && *mail.SenderName != "" {
		from = fmt.Sprintf("%s <%s>", *mail.SenderName, mail.SenderEmail)
	}

	const tsLayout = "2006-01-02 15:04:05"
	pairs := []output.KVPair{
		{Key: "ID", Value: mail.ID},
		{Key: "Message ID", Value: mail.MessageID},
		{Key: "From", Value: from},
	}
	if mail.ToAddresses != nil {
		pairs = append(pairs, output.KVPair{Key: "To", Value: *mail.ToAddresses})
	}
	if mail.CCAddresses != nil {
		pairs = append(pairs, output.KVPair{Key: "CC", Value: *mail.CCAddresses})
	}
	if mail.BCCAddresses != nil {
		pairs = append(pairs, output.KVPair{Key: "BCC", Value: *mail.BCCAddresses})
	}
	if mail.Subject != nil {
		pairs = append(pairs, output.KVPair{Key: "Subject", Value: *mail.Subject})
	}
	pairs = append(pairs, output.KVPair{Key: "Received", Value: mailboxReceived(mail).Format(tsLayout)})
	pairs = append(pairs, output.KVPair{Key: "Processed", Value: yesNo(mail.Processed)})
	if mail.ProcessedAt != nil {
		pairs = append(pairs, output.KVPair{Key: "Processed At", Value: mail.ProcessedAt.Format(tsLayout)})
	}
	if mail.DocumentID != nil {
		pairs = append(pairs, output.KVPair{Key: "Document", Value: *mail.DocumentID})
	}
	if mail.ErrorMessage != nil {
		pairs = append(pairs, output.KVPair{Key: "Error", Value: *mail.ErrorMessage})
	}
	if err := r.KeyValue(pairs); err != nil {
		return err
	}

	if len(mail.Attachments) == 0 {
		return nil
	}

	fmt.Fprintln(r.Writer())
	headers := []string{"Filename", "Type", "Size"}
	var rows [][]string
	for _, a := range mail.Attachments {
		size := "-"
		if a.Size != nil {
			size = formatFileSize(*a.Size)
		}
		rows = append(rows, []string{a.Filename, deref(a.ContentType, "-"), size})
	}
	return r.Table(headers, rows)
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// mailboxReceived returns received_at, falling back to created_at as the API
// does for its date filters.
func mailboxReceived(mail *client.InboundEmailResponse) time.Time {
	if mail.ReceivedAt != nil {
		return *mail.ReceivedAt
	}
	return mail.CreatedAt
}

// --- mailbox get ---

func newMailboxGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <email-id>",
		Short:   "Display inbound email details",
		Example: "  peppol mailbox get mail123\n  peppol mailbox get mail123 --json",
		Args:    cobra.ExactArgs(1),
		RunE:    runMailboxGet,
	}
}

func runMailboxGet(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	mail, err := c.GetMailboxEmail(args[0])
	if err != nil {
		return handleMailboxError(err, "inbound email")
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(mail)
	}

	return renderMailboxEmail(r, mail)
}

// --- mailbox attachment ---

func newMailboxAttachmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachment <email-id> <filename>",
		Aliases: []string{"att"},
		Short:   "Download an inbound email attachment",
		Example: "  peppol mailbox attachment mail123 invoice.pdf -o invoice.pdf\n  peppol mailbox attachment mail123 invoice.xml > invoice.xml",
		Args:    cobra.ExactArgs(2),
		RunE:    runMailboxAttachment,
	}
	cmd.Flags().StringP("output", "o", "", "Write attachment to file instead of stdout")
	return cmd
}

func runMailboxAttachment(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	att, err := c.DownloadMailboxAttachment(args[0], args[1])
	if err != nil {
		return handleMailboxError(err, "inbound email or attachment")
	}

	outputPath, _ := cmd.Flags().GetString("output")
	return writeMailboxAttachment(output.FromContext(cmd.Context()), att, outputPath)
}

// writeMailboxAttachment writes the attachment to outputPath, or the raw
// bytes to stdout when no path is given, also in JSON mode.
func writeMailboxAttachment(r *output.Renderer, att *client.MailboxAttachment, outputPath string) error {
	if outputPath == "" {
		if _, err := r.Writer().Write(att.Content); err != nil {
			return &ExitError{Err: fmt.Errorf("writing output: %w", err), Code: 1}
		}
		return nil
	}

	if err := os.WriteFile(outputPath, att.Content, 0o644); err != nil {
		return &ExitError{Err: fmt.Errorf("writing output file: %w", err), Code: 1}
	}

	if r.IsJSON() {
		return r.JSON(map[string]any{
			"output":       outputPath,
			"content_type": att.ContentType,
			"size":         len(att.Content),
		})
	}

	r.Success(fmt.Sprintf("Attachment written to %s (%s)", outputPath, formatFileSize(len(att.Content))))
	return nil
}

// --- mailbox reprocess ---

func newMailboxReprocessCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "reprocess <email-id>",
		Short:   "Retry processing of a failed inbound email",
		Example: "  peppol mailbox reprocess mail123",
		Args:    cobra.ExactArgs(1),
		RunE:    runMailboxReprocess,
	}
}

func runMailboxReprocess(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	mail, err := c.ReprocessMailboxEmail(args[0])
	if err != nil {
		return handleMailboxError(err, "inbound email")
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(mail)
	}

	r.Success("Reprocessing started.")
	fmt.Fprintln(r.Writer())
	return renderMailboxEmail(r, mail)
}

// handleMailboxError converts client errors to ExitErrors. resource names
// what a 404 refers to.
func handleMailboxError(err error, resource string) error {
	if errors.Is(err, client.ErrNotFound) {
		return &ExitError{
			Err:  fmt.Errorf("%s not found. List emails with 'peppol mailbox list'", resource),
			Code: 4,
		}
	}
	if errors.Is(err, client.ErrUnauthorized) {
		return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
	}
	return &ExitError{Err: err, Code: 1}
}
