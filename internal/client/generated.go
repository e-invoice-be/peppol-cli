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
