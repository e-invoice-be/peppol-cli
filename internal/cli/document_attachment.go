package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAttachmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "attachment",
		Aliases: []string{"att"},
		Short:   "Manage document attachments",
	}

	cmd.AddCommand(newAttachmentListCmd())
	cmd.AddCommand(newAttachmentGetCmd())
	cmd.AddCommand(newAttachmentAddCmd())
	cmd.AddCommand(newAttachmentDeleteCmd())

	return cmd
}

func newAttachmentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <document-id>",
		Short: "List attachments for a document",
		Args:  cobra.ExactArgs(1),
		RunE:  runAttachmentList,
	}
}

func runAttachmentList(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
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
		Use:   "get <document-id> <attachment-id>",
		Short: "Show attachment details or download with --output",
		Args:  cobra.ExactArgs(2),
		RunE:  runAttachmentGet,
	}

	cmd.Flags().StringP("output", "o", "", "Download attachment to file")

	return cmd
}

func runAttachmentGet(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	att, err := c.GetAttachment(args[0], args[1])
	if err != nil {
		return handleAttachmentError(err)
	}

	outputPath, _ := cmd.Flags().GetString("output")
	if outputPath != "" {
		return downloadAttachment(att, outputPath)
	}

	r := output.FromContext(cmd.Context())

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

func downloadAttachment(att *client.DocumentAttachment, outputPath string) error {
	if att.FileURL == nil {
		return &ExitError{Err: fmt.Errorf("attachment has no download URL"), Code: 1}
	}

	resp, err := http.Get(*att.FileURL)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("downloading attachment: %w", err), Code: 1}
	}
	defer resp.Body.Close()

	f, err := os.Create(outputPath)
	if err != nil {
		return &ExitError{Err: fmt.Errorf("creating output file: %w", err), Code: 1}
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return &ExitError{Err: fmt.Errorf("writing file: %w", err), Code: 1}
	}

	return nil
}

func newAttachmentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <document-id> <file>",
		Short: "Upload a file as an attachment",
		Args:  cobra.ExactArgs(2),
		RunE:  runAttachmentAdd,
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

	c := client.NewClient(apiKey)
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
		Use:   "delete <document-id> <attachment-id>",
		Short: "Delete an attachment",
		Args:  cobra.ExactArgs(2),
		RunE:  runAttachmentDelete,
	}

	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	return cmd
}

func runAttachmentDelete(cmd *cobra.Command, args []string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		fmt.Fprintf(cmd.OutOrStdout(), "Delete attachment %s from document %s? [y/N] ", args[1], args[0])
		reader := bufio.NewReader(cmd.InOrStdin())
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
	}

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	_, err = c.DeleteAttachment(args[0], args[1])
	if err != nil {
		return handleAttachmentError(err)
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(map[string]bool{"deleted": true})
	}

	r.Success("Attachment deleted.")
	return nil
}

func handleAttachmentError(err error) error {
	if errors.Is(err, client.ErrNotFound) {
		return &ExitError{Err: fmt.Errorf("not found"), Code: 4}
	}
	if errors.Is(err, client.ErrUnauthorized) {
		return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
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
