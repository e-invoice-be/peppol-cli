package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/e-invoice-be/peppol-cli/internal/client"
	"github.com/e-invoice-be/peppol-cli/internal/output"
)

var (
	statsFrom        string
	statsTo          string
	statsAggregation string
)

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   "Display usage statistics",
		Example: "  peppol stats\n  peppol stats --from 2024-01-01 --to 2024-12-31\n  peppol stats --aggregation MONTH --json",
		RunE:    runStats,
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

	c := client.NewClient(apiKey, clientOpts()...).WithContext(cmd.Context())
	stats, err := c.GetStats(statsFrom, statsTo, statsAggregation)
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			return &ExitError{Err: errors.New("authentication failed (invalid API key)"), Code: 2}
		}
		return &ExitError{Err: err, Code: 1}
	}

	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(stats)
	}

	pairs := []output.KVPair{
		{Key: "Period", Value: fmt.Sprintf("%s to %s", stats.PeriodStart, stats.PeriodEnd)},
		{Key: "Aggregation", Value: string(stats.Aggregation)},
		{Key: "Total Days", Value: strconv.Itoa(stats.TotalDays)},
		{Key: "Average Daily Usage", Value: fmt.Sprintf("%.2f", stats.AverageDailyUsage)},
	}
	if stats.BudgetEstimationDays != nil {
		pairs = append(pairs, output.KVPair{Key: "Budget Estimation", Value: fmt.Sprintf("%.0f days", *stats.BudgetEstimationDays)})
	}
	if err := r.KeyValue(pairs); err != nil {
		return err
	}

	if len(stats.Actions) == 0 || r.IsQuiet() {
		return nil
	}

	fmt.Fprintln(r.Writer())
	return r.Table([]string{"Date", "Sent", "Received"}, statsActionRows(stats.Actions))
}

// statsActionRows pivots actions into one row per date, sorted by date:
// date, documents sent, documents received. Other actions only add a row.
func statsActionRows(actions []client.ActionStats) [][]string {
	type counts struct{ sent, received int }

	byDate := make(map[string]*counts)
	for _, a := range actions {
		c, ok := byDate[a.StatDate]
		if !ok {
			c = &counts{}
			byDate[a.StatDate] = c
		}
		switch a.Action {
		case client.ActionDocumentSent:
			c.sent += a.Count
		case client.ActionDocumentReceived:
			c.received += a.Count
		}
	}

	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	rows := make([][]string, 0, len(dates))
	for _, d := range dates {
		c := byDate[d]
		rows = append(rows, []string{d, strconv.Itoa(c.sent), strconv.Itoa(c.received)})
	}
	return rows
}
