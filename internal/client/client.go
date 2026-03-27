package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const DefaultBaseURL = "https://api.e-invoice.be"

var (
	ErrUnauthorized = errors.New("authentication failed (invalid or missing API key)")
	ErrNotFound     = errors.New("resource not found")
)

// APIError represents a non-auth API error with status code.
type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Detail)
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default API base URL (for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// Client wraps the e-invoice.be API with Bearer token authentication.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client with the given API key.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Transport: &authTransport{apiKey: apiKey, base: http.DefaultTransport},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	// Ensure the transport has the right key if httpClient wasn't overridden.
	if c.httpClient.Transport == nil {
		c.httpClient.Transport = &authTransport{apiKey: c.apiKey, base: http.DefaultTransport}
	}
	return c
}

// GetMe calls GET /api/me/ and returns the tenant info.
func (c *Client) GetMe() (*TenantPublic, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/me/", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var tenant TenantPublic
		if err := json.Unmarshal(body, &tenant); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &tenant, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// GetStats calls GET /api/stats and returns usage statistics.
func (c *Client) GetStats(startDate, endDate, aggregation string) (*StatsResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	if startDate != "" {
		q.Set("start_date", startDate)
	}
	if endDate != "" {
		q.Set("end_date", endDate)
	}
	if aggregation != "" {
		q.Set("aggregation", aggregation)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var stats StatsResponse
		if err := json.Unmarshal(body, &stats); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &stats, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// GetDocument calls GET /api/documents/{document_id} and returns the document details.
func (c *Client) GetDocument(documentID string) (*DocumentResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/documents/"+documentID, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var doc DocumentResponse
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &doc, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// GetDocumentTimeline calls GET /api/documents/{document_id}/timeline and returns the timeline.
func (c *Client) GetDocumentTimeline(documentID string) (*DocumentTimeline, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/documents/"+documentID+"/timeline", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var timeline DocumentTimeline
		if err := json.Unmarshal(body, &timeline); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &timeline, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// DocumentListParams holds query parameters for document list endpoints.
type DocumentListParams struct {
	Type      string
	Sender    string
	Receiver  string
	State     string
	FromDate  string
	ToDate    string
	Search    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

func (c *Client) listDocuments(path string, params DocumentListParams) (*PaginatedDocuments, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	if params.Type != "" {
		q.Set("type", params.Type)
	}
	if params.Sender != "" {
		q.Set("sender", params.Sender)
	}
	if params.Receiver != "" {
		q.Set("receiver", params.Receiver)
	}
	if params.State != "" {
		q.Set("state", params.State)
	}
	if params.FromDate != "" {
		q.Set("date_from", params.FromDate)
	}
	if params.ToDate != "" {
		q.Set("date_to", params.ToDate)
	}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if params.SortBy != "" {
		q.Set("sort_by", params.SortBy)
	}
	if params.SortOrder != "" {
		q.Set("sort_order", params.SortOrder)
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(params.PageSize))
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result PaginatedDocuments
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// ListInbox calls GET /api/inbox/ and returns paginated received documents.
func (c *Client) ListInbox(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/inbox/", params)
}

// ListInboxInvoices calls GET /api/inbox/invoices and returns received invoices.
func (c *Client) ListInboxInvoices(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/inbox/invoices", params)
}

// ListInboxCreditNotes calls GET /api/inbox/credit-notes and returns received credit notes.
func (c *Client) ListInboxCreditNotes(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/inbox/credit-notes", params)
}

// ListOutbox calls GET /api/outbox/ and returns paginated sent documents.
func (c *Client) ListOutbox(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/outbox/", params)
}

// ListOutboxDrafts calls GET /api/outbox/drafts and returns outbox draft documents.
func (c *Client) ListOutboxDrafts(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/outbox/drafts", params)
}

// ListDrafts calls GET /api/drafts/ and returns all draft documents.
func (c *Client) ListDrafts(params DocumentListParams) (*PaginatedDocuments, error) {
	return c.listDocuments("/api/drafts/", params)
}

// LookupPeppolID calls GET /api/lookup and returns Peppol participant information.
func (c *Client) LookupPeppolID(peppolID string) (*PeppolIdLookupResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/lookup", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("peppol_id", peppolID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result PeppolIdLookupResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// SearchPeppolParticipants calls GET /api/lookup/participants and returns matching participants.
func (c *Client) SearchPeppolParticipants(query, countryCode string) (*PeppolSearchResult, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/lookup/participants", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("query", query)
	if countryCode != "" {
		q.Set("country_code", countryCode)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result PeppolSearchResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// ValidatePeppolID calls GET /api/validate/peppol-id and returns validation results.
func (c *Client) ValidatePeppolID(peppolID string) (*PeppolIdValidationResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/validate/peppol-id", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("peppol_id", peppolID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result PeppolIdValidationResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// MaskKey returns a masked version of an API key for display.
func MaskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return "****..." + key[len(key)-4:]
}

// authTransport injects the Bearer token into every request.
type authTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(req2)
}
