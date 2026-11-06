package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

func newAttachmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachment",
		Aliases: []string{"att"},
		Short:   "Manage document attachments",
		Example: "  peppol document attachment list <document-id>\n  peppol doc att add <document-id> file.pdf",
	}

	cmd.AddCommand(newAttachmentListCmd())
	cmd.AddCommand(newAttachmentGetCmd())
	cmd.AddCommand(newAttachmentAddCmd())
	cmd.AddCommand(newAttachmentDeleteCmd())

	return cmd
}

func newAttachmentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <document-id>",
		Short:   "List attachments for a document",
		Example: "  peppol document attachment list abc123",
		Args:    cobra.ExactArgs(1),
		RunE:    runAttachmentList,
	}
}

func runAttachmentList(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	attachments, err := c.ListAttachments(args[0])
	if err != nil {
		return handleAttachmentError(err)
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(attachments)
	}

	return renderAttachmentList(r, attachments)
}

func renderAttachmentList(r *output.Renderer, attachments []client.DocumentAttachment) error {
	if len(attachments) == 0 {
		fmt.Fprintln(r.Writer(), "No attachments.")
		return nil
	}

	headers := []string{"ID", "Filename", "Type", "Size"}
	var rows [][]string
	for _, a := range attachments {
		rows = append(rows, []string{a.ID, a.FileName, a.FileType, formatFileSize(a.FileSize)})
	}
	return r.Table(headers, rows)
}

func newAttachmentGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <document-id> <attachment-id>",
		Short:   "Show attachment details or download with --output",
		Example: "  peppol document attachment get abc123 att456\n  peppol document attachment get abc123 att456 -o file.pdf",
		Args:    cobra.ExactArgs(2),
		RunE:    runAttachmentGet,
	}

	cmd.Flags().StringP("output", "o", "", "Download attachment to file")

	return cmd
}

func runAttachmentGet(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	att, err := c.GetAttachment(args[0], args[1])
	if err != nil {
		return handleAttachmentError(err)
	}

	outputPath, _ := cmd.Flags().GetString("output")
	r := output.FromContext(cmd.Context())

	if outputPath != "" {
		n, err := downloadAttachment(cmd.Context(), att, outputPath)
		if err != nil {
			return err
		}
		r.Success(fmt.Sprintf("Attachment written to %s (%s)", outputPath, formatFileSize(int(n))))
		return nil
	}

	if r.IsJSON() {
		return r.JSON(att)
	}

	return r.KeyValue([]output.KVPair{
		{Key: "ID", Value: att.ID},
		{Key: "Filename", Value: att.FileName},
		{Key: "Type", Value: att.FileType},
		{Key: "Size", Value: formatFileSize(att.FileSize)},
	})
}

// downloadAttachment saves the attachment's signed URL to outputPath and
// returns the number of bytes written.
func downloadAttachment(ctx context.Context, att *client.DocumentAttachment, outputPath string) (int64, error) {
	if att.FileURL == nil {
		return 0, &ExitError{Err: errors.New("attachment has no download URL"), Code: 1}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *att.FileURL, http.NoBody)
	if err != nil {
		return 0, &ExitError{Err: fmt.Errorf("downloading attachment: %w", err), Code: 1}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, &ExitError{Err: fmt.Errorf("downloading attachment: %w", err), Code: 1}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, &ExitError{Err: fmt.Errorf("downloading attachment: HTTP %d", resp.StatusCode), Code: 1}
	}

	f, err := os.Create(outputPath) //nolint:gosec // G304: --output path is supplied by the user on purpose
	if err != nil {
		return 0, &ExitError{Err: fmt.Errorf("creating output file: %w", err), Code: 1}
	}

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return 0, &ExitError{Err: fmt.Errorf("writing file: %w", err), Code: 1}
	}

	if err := f.Close(); err != nil {
		return 0, &ExitError{Err: fmt.Errorf("closing output file: %w", err), Code: 1}
	}

	return n, nil
}

func newAttachmentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "add <document-id> <file>",
		Short:   "Upload a file as an attachment",
		Example: "  peppol document attachment add abc123 invoice.pdf",
		Args:    cobra.ExactArgs(2),
		RunE:    runAttachmentAdd,
	}
}

func runAttachmentAdd(cmd *cobra.Command, args []string) error {
	filePath := args[1]
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ExitError{Err: fmt.Errorf("file not found: %s", filePath), Code: 1}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	att, err := c.AddAttachment(args[0], filePath)
	if err != nil {
		return handleAttachmentError(err)
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(att)
	}

	r.Success(fmt.Sprintf("Uploaded %s (%s)", att.FileName, formatFileSize(att.FileSize)))
	return nil
}

func newAttachmentDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <document-id> <attachment-id>",
		Short:   "Delete an attachment",
		Example: "  peppol document attachment delete abc123 att456 --yes",
		Args:    cobra.ExactArgs(2),
		RunE:    runAttachmentDelete,
	}

	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	return cmd
}

func runAttachmentDelete(cmd *cobra.Command, args []string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && !confirm(cmd, fmt.Sprintf("Delete attachment %s from document %s?", args[1], args[0])) {
		return nil
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	result, err := c.DeleteAttachment(args[0], args[1])
	if err != nil {
		return handleAttachmentError(err)
	}

	r := output.FromContext(cmd.Context())

	// Same output as "document delete": the API's is_deleted.
	if r.IsJSON() {
		return r.JSON(result)
	}

	if result.IsDeleted {
		r.Success("Attachment deleted.")
	}
	return nil
}

func handleAttachmentError(err error) error {
	if errors.Is(err, client.ErrNotFound) {
		return &ExitError{Err: errors.New("not found"), Code: 4}
	}
	if errors.Is(err, client.ErrUnauthorized) {
		return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
	}
	return &ExitError{Err: err, Code: 1}
}

func formatFileSize(bytes int) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
