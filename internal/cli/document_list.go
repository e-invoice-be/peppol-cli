package cli

import (
	"fmt"
	"strings"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

var validSortFields = []string{"created_at", "invoice_date", "due_date", "invoice_total", "customer_name", "vendor_name", "invoice_id"}
var validSortOrders = []string{"asc", "desc"}

// CounterpartyMode determines which name column to show in the table.
type CounterpartyMode int

const (
	CounterpartySeller CounterpartyMode = iota // inbox: show seller
	CounterpartyBuyer                          // outbox/drafts: show buyer
)

// DocumentListFlags holds the common filter/pagination flags for document list commands.
type DocumentListFlags struct {
	DocType   string
	From      string
	To        string
	Search    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
	Sender    string
	Receiver  string
	State     string
}

// BindCommonFlags adds shared pagination and filter flags to a command.
func (f *DocumentListFlags) BindCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.DocType, "type", "", "Filter by document type (invoice, credit-note)")
	cmd.Flags().StringVar(&f.From, "from", "", "Start date (yyyy-mm-dd)")
	cmd.Flags().StringVar(&f.To, "to", "", "End date (yyyy-mm-dd)")
	cmd.Flags().StringVar(&f.Search, "search", "", "Search term")
	cmd.Flags().StringVar(&f.SortBy, "sort-by", "", "Sort field ("+strings.Join(validSortFields, ", ")+")")
	cmd.Flags().StringVar(&f.SortOrder, "sort-order", "", "Sort direction (asc, desc)")
	cmd.Flags().IntVar(&f.Page, "page", 1, "Page number")
	cmd.Flags().IntVar(&f.PageSize, "page-size", 20, "Results per page")
}

// BindSenderFlag adds the --sender flag (inbox only).
func (f *DocumentListFlags) BindSenderFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Sender, "sender", "", "Filter by sender")
}

// BindReceiverFlag adds the --receiver flag (outbox only).
func (f *DocumentListFlags) BindReceiverFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Receiver, "receiver", "", "Filter by receiver")
}

// BindStateFlag adds the --state flag (drafts only).
func (f *DocumentListFlags) BindStateFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.State, "state", "", "Filter by state")
}

// Validate checks that flag values are valid.
func (f *DocumentListFlags) Validate() error {
	if f.SortBy != "" {
		valid := false
		for _, v := range validSortFields {
			if f.SortBy == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid --sort-by %q, must be one of: %s", f.SortBy, strings.Join(validSortFields, ", "))
		}
	}
	if f.SortOrder != "" {
		valid := false
		for _, v := range validSortOrders {
			if f.SortOrder == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid --sort-order %q, must be one of: %s", f.SortOrder, strings.Join(validSortOrders, ", "))
		}
	}
	return nil
}

// ToParams converts flags to client.DocumentListParams.
func (f *DocumentListFlags) ToParams() client.DocumentListParams {
	return client.DocumentListParams{
		Type:      f.DocType,
		Sender:    f.Sender,
		Receiver:  f.Receiver,
		State:     f.State,
		FromDate:  f.From,
		ToDate:    f.To,
		Search:    f.Search,
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
		Page:      f.Page,
		PageSize:  f.PageSize,
	}
}

// renderDocumentList renders a paginated document list as table or JSON.
func renderDocumentList(cmd *cobra.Command, result *client.PaginatedDocuments, mode CounterpartyMode) error {
	r := output.FromContext(cmd.Context())

	if r.IsJSON() {
		return r.JSON(result)
	}

	if len(result.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No documents found.")
		return nil
	}

	headers := []string{"ID", "TYPE", "STATE", "INVOICE #", "COUNTERPARTY", "TOTAL", "DATE"}
	rows := make([][]string, 0, len(result.Items))
	for _, doc := range result.Items {
		counterparty := deref(doc.VendorName, "")
		if mode == CounterpartyBuyer {
			counterparty = deref(doc.CustomerName, "")
		}
		rows = append(rows, []string{
			doc.ID,
			string(doc.DocumentType),
			output.StatusBadge(string(doc.State)),
			deref(doc.InvoiceID, ""),
			counterparty,
			deref(doc.InvoiceTotal, ""),
			deref(doc.InvoiceDate, ""),
		})
	}

	if err := r.Table(headers, rows); err != nil {
		return err
	}
	r.Pagination(result.Page, result.PageSize, result.Total)
	return nil
}
