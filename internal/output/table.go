package output

import (
	"fmt"
	"math"

	"github.com/olekukonko/tablewriter"
)

// Table renders tabular data with headers and rows.
func (r *Renderer) Table(headers []string, rows [][]string) error {
	if r.quiet {
		return nil
	}

	tw := tablewriter.NewTable(r.w)

	hdrs := make([]any, len(headers))
	for i, h := range headers {
		hdrs[i] = h
	}
	tw.Header(hdrs...)

	for _, row := range rows {
		vals := make([]any, len(row))
		for i, v := range row {
			vals[i] = v
		}
		_ = tw.Append(vals...)
	}

	return tw.Render()
}

// Pagination prints a pagination footer below output.
func (r *Renderer) Pagination(page, pageSize, total int) {
	if r.quiet {
		return
	}
	if total == 0 {
		return
	}

	start := (page-1)*pageSize + 1
	end := page * pageSize
	if end > total {
		end = total
	}
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	msg := fmt.Sprintf("Showing %d-%d of %d documents (page %d/%d)", start, end, total, page, totalPages)
	if r.color {
		msg = LabelStyle.Render(msg)
	}
	fmt.Fprintln(r.w, msg)
}
