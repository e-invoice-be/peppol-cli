// Code generated from api/openapi.json (manually, OpenAPI 3.1 not supported by oapi-codegen).
// Regenerate when the spec changes.

package client

import (
	"encoding/json"
	"time"
)

// TenantPublic represents the response from GET /api/me/.
type TenantPublic struct {
	Name                string     `json:"name"`
	Description         *string    `json:"description,omitempty"`
	Plan                string     `json:"plan,omitempty"`
	CreditBalance       int        `json:"credit_balance,omitempty"`
	PeppolIDs           []string   `json:"peppol_ids,omitempty"`
	IBANs               []string   `json:"ibans,omitempty"`
	CompanyNumber       *string    `json:"company_number,omitempty"`
	CompanyTaxID        *string    `json:"company_tax_id,omitempty"`
	CompanyName         *string    `json:"company_name,omitempty"`
	CompanyAddress      *string    `json:"company_address,omitempty"`
	CompanyZip          *string    `json:"company_zip,omitempty"`
	CompanyCity         *string    `json:"company_city,omitempty"`
	CompanyCountry      *string    `json:"company_country,omitempty"`
	CompanyEmail        *string    `json:"company_email,omitempty"`
	SMPRegistration     *bool      `json:"smp_registration,omitempty"`
	SMPRegistrationDate *time.Time `json:"smp_registration_date,omitempty"`
	BCCRecipientEmail   *string    `json:"bcc_recipient_email,omitempty"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Detail string `json:"detail"`
}

// StatsAggregationType represents the aggregation period for stats.
type StatsAggregationType string

const (
	StatsAggregationDay   StatsAggregationType = "DAY"
	StatsAggregationWeek  StatsAggregationType = "WEEK"
	StatsAggregationMonth StatsAggregationType = "MONTH"
)

// ActionType represents the type of document action.
type ActionType string

const (
	ActionDocumentSent     ActionType = "DOCUMENT_SENT"
	ActionDocumentReceived ActionType = "DOCUMENT_RECEIVED"
)

// ActionStats represents a single action statistic entry.
type ActionStats struct {
	Action   ActionType `json:"action"`
	StatDate string     `json:"stat_date"`
	Count    int        `json:"count"`
}

// DocumentType represents the type of document.
type DocumentType string

const (
	DocumentTypeInvoice              DocumentType = "INVOICE"
	DocumentTypeCreditNote           DocumentType = "CREDIT_NOTE"
	DocumentTypeDebitNote            DocumentType = "DEBIT_NOTE"
	DocumentTypeSelfbillingInvoice   DocumentType = "SELFBILLING_INVOICE"
	DocumentTypeSelfbillingCreditNote DocumentType = "SELFBILLING_CREDIT_NOTE"
)

// DocumentState represents the processing state of a document.
type DocumentState string

const (
	DocumentStateDraft    DocumentState = "DRAFT"
	DocumentStateTransit  DocumentState = "TRANSIT"
	DocumentStateFailed   DocumentState = "FAILED"
	DocumentStateSent     DocumentState = "SENT"
	DocumentStateReceived DocumentState = "RECEIVED"
)

// DocumentDirection represents the direction of a document.
type DocumentDirection string

const (
	DocumentDirectionInbound  DocumentDirection = "INBOUND"
	DocumentDirectionOutbound DocumentDirection = "OUTBOUND"
)

// TimelineEventType represents the type of timeline event.
type TimelineEventType string

const (
	TimelineEmailReceived   TimelineEventType = "email_received"
	TimelineEmailProcessed  TimelineEventType = "email_processed"
	TimelineDocumentCreated TimelineEventType = "document_created"
	TimelineSendAttempted   TimelineEventType = "send_attempted"
	TimelineSendFailed      TimelineEventType = "send_failed"
	TimelineSendSuccess     TimelineEventType = "send_success"
	TimelineReceiveSuccess  TimelineEventType = "receive_success"
	TimelineMLRReceived     TimelineEventType = "mlr_received"
	TimelineIMRReceived     TimelineEventType = "imr_received"
)

// DocumentResponse represents the response from GET /api/documents/{document_id}.
type DocumentResponse struct {
	ID                        string            `json:"id"`
	CreatedAt                 time.Time         `json:"created_at"`
	DocumentType              DocumentType      `json:"document_type,omitempty"`
	State                     DocumentState     `json:"state,omitempty"`
	Direction                 DocumentDirection `json:"direction,omitempty"`
	CustomerName              *string           `json:"customer_name,omitempty"`
	CustomerID                *string           `json:"customer_id,omitempty"`
	CustomerEmail             *string           `json:"customer_email,omitempty"`
	CustomerTaxID             *string           `json:"customer_tax_id,omitempty"`
	CustomerCompanyID         *string           `json:"customer_company_id,omitempty"`
	CustomerPeppolID          *string           `json:"customer_peppol_id,omitempty"`
	CustomerAddress           *string           `json:"customer_address,omitempty"`
	CustomerAddressRecipient  *string           `json:"customer_address_recipient,omitempty"`
	VendorName                *string           `json:"vendor_name,omitempty"`
	VendorEmail               *string           `json:"vendor_email,omitempty"`
	VendorAddress             *string           `json:"vendor_address,omitempty"`
	VendorAddressRecipient    *string           `json:"vendor_address_recipient,omitempty"`
	VendorTaxID               *string           `json:"vendor_tax_id,omitempty"`
	VendorCompanyID           *string           `json:"vendor_company_id,omitempty"`
	PurchaseOrder             *string           `json:"purchase_order,omitempty"`
	InvoiceID                 *string           `json:"invoice_id,omitempty"`
	InvoiceDate               *string           `json:"invoice_date,omitempty"`
	DueDate                   *string           `json:"due_date,omitempty"`
	BillingAddress            *string           `json:"billing_address,omitempty"`
	BillingAddressRecipient   *string           `json:"billing_address_recipient,omitempty"`
	ShippingAddress           *string           `json:"shipping_address,omitempty"`
	ShippingAddressRecipient  *string           `json:"shipping_address_recipient,omitempty"`
	RemittanceAddress         *string           `json:"remittance_address,omitempty"`
	RemittanceAddressRecipient *string          `json:"remittance_address_recipient,omitempty"`
	ServiceAddress            *string           `json:"service_address,omitempty"`
	ServiceAddressRecipient   *string           `json:"service_address_recipient,omitempty"`
	ServiceStartDate          *string           `json:"service_start_date,omitempty"`
	ServiceEndDate            *string           `json:"service_end_date,omitempty"`
	Currency                  string            `json:"currency,omitempty"`
	TaxCode                   *string           `json:"tax_code,omitempty"`
	Vatex                     *string           `json:"vatex,omitempty"`
	VatexNote                 *string           `json:"vatex_note,omitempty"`
	Subtotal                  *string           `json:"subtotal,omitempty"`
	TotalDiscount             *string           `json:"total_discount,omitempty"`
	TotalTax                  *string           `json:"total_tax,omitempty"`
	InvoiceTotal              *string           `json:"invoice_total,omitempty"`
	AmountDue                 *string           `json:"amount_due,omitempty"`
	Note                      *string           `json:"note,omitempty"`
	PaymentTerm               *string           `json:"payment_term,omitempty"`
	PaymentDetails            []PaymentDetail   `json:"payment_details,omitempty"`
	TaxDetails                json.RawMessage   `json:"tax_details,omitempty"`
	Items                     []LineItem        `json:"items,omitempty"`
	Attachments               json.RawMessage   `json:"attachments,omitempty"`
	Allowances                json.RawMessage   `json:"allowances,omitempty"`
	Charges                   json.RawMessage   `json:"charges,omitempty"`
}

// LineItem represents a single line item in a document.
type LineItem struct {
	Description *string `json:"description,omitempty"`
	Quantity    *string `json:"quantity,omitempty"`
	Unit        *string `json:"unit,omitempty"`
	UnitPrice   *string `json:"unit_price,omitempty"`
	Amount      *string `json:"amount,omitempty"`
	TaxRate     *string `json:"tax_rate,omitempty"`
	Tax         *string `json:"tax,omitempty"`
	ProductCode *string `json:"product_code,omitempty"`
	Date        *string `json:"date,omitempty"`
}

// PaymentDetail represents payment information for a document.
type PaymentDetail struct {
	IBAN             *string `json:"iban,omitempty"`
	SWIFT            *string `json:"swift,omitempty"`
	BankAccountNumber *string `json:"bank_account_number,omitempty"`
	PaymentReference *string `json:"payment_reference,omitempty"`
}

// DocumentTimeline represents the response from GET /api/documents/{document_id}/timeline.
type DocumentTimeline struct {
	DocumentID string          `json:"document_id"`
	Events     []TimelineEvent `json:"events"`
}

// TimelineEvent represents a single event in a document's timeline.
type TimelineEvent struct {
	EventType TimelineEventType `json:"event_type"`
	Timestamp time.Time         `json:"timestamp"`
	ID        *string           `json:"id,omitempty"`
	Details   map[string]any    `json:"details,omitempty"`
}

// StatsResponse represents the response from GET /api/stats.
type StatsResponse struct {
	TenantID             string               `json:"tenant_id"`
	PeriodStart          string               `json:"period_start"`
	PeriodEnd            string               `json:"period_end"`
	Aggregation          StatsAggregationType  `json:"aggregation"`
	Actions              []ActionStats         `json:"actions"`
	TotalDays            int                   `json:"total_days"`
	AverageDailyUsage    float64              `json:"average_daily_usage"`
	BudgetEstimationDays *float64             `json:"budget_estimation_days,omitempty"`
}
