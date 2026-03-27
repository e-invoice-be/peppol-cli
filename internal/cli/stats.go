package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/spf13/cobra"
)

var (
	statsFrom        string
	statsTo          string
	statsAggregation string
)

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display usage statistics",
		RunE:  runStats,
	}
	cmd.Flags().StringVar(&statsFrom, "from", "", "Start date (yyyy-mm-dd)")
	cmd.Flags().StringVar(&statsTo, "to", "", "End date (yyyy-mm-dd)")
	cmd.Flags().StringVar(&statsAggregation, "aggregation", "", "Aggregation period: DAY, WEEK, or MONTH")
	return cmd
}

func runStats(cmd *cobra.Command, args []string) error {
	apiKey, err := resolveKey()
	if err != nil {
		return err
	}

	c := client.NewClient(apiKey)
	stats, err := c.GetStats(statsFrom, statsTo, statsAggregation)
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: fmt.Errorf("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	if flags.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Period:              %s to %s\n", stats.PeriodStart, stats.PeriodEnd)
	fmt.Fprintf(w, "Aggregation:         %s\n", stats.Aggregation)
	fmt.Fprintf(w, "Total Days:          %d\n", stats.TotalDays)
	fmt.Fprintf(w, "Average Daily Usage: %.2f\n", stats.AverageDailyUsage)
	if stats.BudgetEstimationDays != nil {
		fmt.Fprintf(w, "Budget Estimation:   %.0f days\n", *stats.BudgetEstimationDays)
	}

	if len(stats.Actions) == 0 {
		return nil
	}

	fmt.Fprintln(w)
	renderActionsTable(cmd, stats.Actions)
	return nil
}

func renderActionsTable(cmd *cobra.Command, actions []client.ActionStats) {
	// Pivot actions by date: map[date]map[action]count
	type dateRow struct {
		date     string
		sent     int
		received int
	}

	byDate := make(map[string]*dateRow)
	for _, a := range actions {
		row, ok := byDate[a.StatDate]
		if !ok {
			row = &dateRow{date: a.StatDate}
			byDate[a.StatDate] = row
		}
		switch a.Action {
		case client.ActionDocumentSent:
			row.sent += a.Count
		case client.ActionDocumentReceived:
			row.received += a.Count
		}
	}

	// Sort dates
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tSENT\tRECEIVED")
	for _, d := range dates {
		row := byDate[d]
		fmt.Fprintf(tw, "%s\t%d\t%d\n", row.date, row.sent, row.received)
	}
	_ = tw.Flush()
}
