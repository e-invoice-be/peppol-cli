package cli

import (
	"fmt"
	"os"

	"github.com/e-invoicebe/peppol-cli/internal/backup"
	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup <dir>",
		Short: "Archive every document, attachment, UBL XML, and timeline to <dir>",
		Long: `Archive every document, attachment, UBL XML, and timeline to <dir>.

The action taken depends on the state of <dir>:

  • empty / missing  — full backup; paginate inbox + outbox + drafts and
                       archive each document's detail, UBL, timeline, and
                       attachments. Writes manifest.json.

  • partial (resume) — manifest.json exists with an unfinished run.
                       Continue from where the prior run was killed; documents
                       already on disk are not re-fetched.

  • complete (top-up)— manifest.json exists with a completed run.
                       Fetch documents whose created_at is greater than the
                       prior run's high-water mark, and re-fetch the detail +
                       timeline for every locally-recorded DRAFT or TRANSIT
                       document. Documents in FAILED, SENT, or RECEIVED are
                       frozen forever.

There is no --resume flag. There is no --since flag. The directory state
decides.

All file writes use temp-file + rename: a kill -9 at any point leaves no
half-written file visible under its final name. The backup is an append-only
archive — server-side deletions are NOT detected.`,
		Example: `  peppol backup ./my-backup
  peppol backup ./my-backup --layout=tree
  peppol backup ./my-backup --concurrency=16`,
		Args: cobra.ExactArgs(1),
		RunE: runBackup,
	}
	cmd.Flags().String("layout", "flat", "On-disk layout: flat (default) or tree")
	cmd.Flags().Int("concurrency", 8, "Per-document workers fetching detail/UBL/timeline/attachments in parallel")
	return cmd
}

func runBackup(cmd *cobra.Command, args []string) error {
	dir := args[0]

	layoutStr, _ := cmd.Flags().GetString("layout")
	layout := backup.Layout(layoutStr)
	if layout != backup.LayoutFlat && layout != backup.LayoutTree {
		return &ExitError{
			Err:  fmt.Errorf("invalid --layout %q: must be flat or tree", layoutStr),
			Code: 1,
		}
	}
	concurrency, _ := cmd.Flags().GetInt("concurrency")

	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())

	res, err := backup.Run(cmd.Context(), backup.Options{
		Dir:         dir,
		Layout:      layout,
		Concurrency: concurrency,
		Quiet:       flags.Quiet,
		APIKey:      apiKey,
		Client:      c,
		Stdout:      cmd.OutOrStdout(),
		Stderr:      os.Stderr,
	})
	if err != nil {
		return &ExitError{Err: err, Code: 1}
	}

	r := output.FromContext(cmd.Context())
	if r.IsJSON() {
		return r.JSON(map[string]any{
			"documents_fetched":   res.DocumentsFetched,
			"documents_refreshed": res.DocumentsRefreshed,
			"documents_skipped":   res.DocumentsSkipped,
			"started_at":          res.StartedAt,
			"completed_at":        res.CompletedAt,
		})
	}

	return r.KeyValue([]output.KVPair{
		{Key: "Directory", Value: dir},
		{Key: "Layout", Value: string(layout)},
		{Key: "New documents", Value: fmt.Sprintf("%d", res.DocumentsFetched)},
		{Key: "Refreshed documents", Value: fmt.Sprintf("%d", res.DocumentsRefreshed)},
	})
}
