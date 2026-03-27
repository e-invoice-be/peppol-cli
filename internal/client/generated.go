// Code generated from api/openapi.json (manually, OpenAPI 3.1 not supported by oapi-codegen).
// Regenerate when the spec changes.

package client

import "time"

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
