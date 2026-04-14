package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/e-invoicebe/peppol-cli/internal/version"
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

// WithVerbose enables HTTP request/response logging to the given writer.
func WithVerbose(w io.Writer) ClientOption {
	return func(c *Client) { c.verbose = w }
}

// Client wraps the e-invoice.be API with Bearer token authentication.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	verbose    io.Writer
	ctx        context.Context
}

// WithContext returns a shallow copy of the client with the given context.
// The context is used for all HTTP requests made by the returned client.
func (c *Client) WithContext(ctx context.Context) *Client {
	c2 := *c
	c2.ctx = ctx
	return &c2
}

// NewClient creates a new API client with the given API key.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		ctx:     context.Background(),
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
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
	if c.verbose != nil {
		c.httpClient.Transport = &verboseTransport{base: c.httpClient.Transport, w: c.verbose}
	}
	return c
}

// GetMe calls GET /api/me/ and returns the tenant info.
func (c *Client) GetMe() (*TenantPublic, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/me/", nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/stats", nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/documents/"+documentID, nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/documents/"+documentID+"/timeline", nil)
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

// ListAttachments calls GET /api/documents/{document_id}/attachments and returns the attachments.
func (c *Client) ListAttachments(documentID string) ([]DocumentAttachment, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/documents/"+documentID+"/attachments", nil)
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
		var attachments []DocumentAttachment
		if err := json.Unmarshal(body, &attachments); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return attachments, nil
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

// GetAttachment calls GET /api/documents/{document_id}/attachments/{attachment_id}.
func (c *Client) GetAttachment(documentID, attachmentID string) (*DocumentAttachment, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/documents/"+documentID+"/attachments/"+attachmentID, nil)
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
		var att DocumentAttachment
		if err := json.Unmarshal(body, &att); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &att, nil
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

// AddAttachment uploads a file as an attachment via POST /api/documents/{document_id}/attachments.
func (c *Client) AddAttachment(documentID, filePath string) (*DocumentAttachment, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("copying file data: %w", err)
	}
	_ = f.Close()
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/api/documents/"+documentID+"/attachments", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		var att DocumentAttachment
		if err := json.Unmarshal(respBody, &att); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &att, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil {
			apiErr.Detail = errResp.Detail
		}
		return nil, apiErr
	}
}

// DeleteAttachment calls DELETE /api/documents/{document_id}/attachments/{attachment_id}.
func (c *Client) DeleteAttachment(documentID, attachmentID string) (*DocumentAttachmentDelete, error) {
	req, err := http.NewRequestWithContext(c.ctx, "DELETE", c.baseURL+"/api/documents/"+documentID+"/attachments/"+attachmentID, nil)
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
		var result DocumentAttachmentDelete
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
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

// CreateDocumentJSON creates a document from a JSON file.
func (c *Client) CreateDocumentJSON(filePath string, constructPDF bool) (*DocumentResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	url := c.baseURL + "/api/documents/"
	if constructPDF {
		url += "?construct_pdf=true"
	}

	req, err := http.NewRequestWithContext(c.ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
	case http.StatusCreated:
		var doc DocumentResponse
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &doc, nil
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

// CreateDocumentFromUBL creates a document from a UBL/XML file.
func (c *Client) CreateDocumentFromUBL(filePath string) (*DocumentResponse, error) {
	resp, err := c.postMultipartFile(c.baseURL+"/api/documents/ubl", filePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		var doc DocumentResponse
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &doc, nil
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

// CreateDocumentFromPDF creates a document from a PDF file.
func (c *Client) CreateDocumentFromPDF(filePath, vendorTaxID, customerTaxID string) (*DocumentCreateFromPdfResponse, error) {
	url := c.baseURL + "/api/documents/pdf"
	q := make([]string, 0)
	if vendorTaxID != "" {
		q = append(q, "vendor_tax_id="+vendorTaxID)
	}
	if customerTaxID != "" {
		q = append(q, "customer_tax_id="+customerTaxID)
	}
	if len(q) > 0 {
		url += "?"
		for i, param := range q {
			if i > 0 {
				url += "&"
			}
			url += param
		}
	}

	resp, err := c.postMultipartFile(url, filePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		var doc DocumentCreateFromPdfResponse
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &doc, nil
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

// SendDocument sends a document via Peppol.
func (c *Client) SendDocument(documentID string, opts SendDocumentOptions) (*DocumentResponse, error) {
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/api/documents/"+documentID+"/send", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	if opts.SenderPeppolScheme != "" {
		q.Set("sender_peppol_scheme", opts.SenderPeppolScheme)
	}
	if opts.SenderPeppolID != "" {
		q.Set("sender_peppol_id", opts.SenderPeppolID)
	}
	if opts.ReceiverPeppolScheme != "" {
		q.Set("receiver_peppol_scheme", opts.ReceiverPeppolScheme)
	}
	if opts.ReceiverPeppolID != "" {
		q.Set("receiver_peppol_id", opts.ReceiverPeppolID)
	}
	if opts.Email != "" {
		q.Set("email", opts.Email)
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

// ValidateDocument validates a document against Peppol BIS Billing 3.0.
func (c *Client) ValidateDocument(documentID string) (*ValidationResponse, error) {
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/api/documents/"+documentID+"/validate", nil)
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
		var val ValidationResponse
		if err := json.Unmarshal(body, &val); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &val, nil
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

// DeleteDocument calls DELETE /api/documents/{document_id} and returns the result.
func (c *Client) DeleteDocument(documentID string) (*DocumentDelete, error) {
	req, err := http.NewRequestWithContext(c.ctx, "DELETE", c.baseURL+"/api/documents/"+documentID, nil)
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
		var result DocumentDelete
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &result, nil
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

// GetDocumentUBL calls GET /api/documents/{document_id}/ubl and returns the UBL metadata.
func (c *Client) GetDocumentUBL(documentID string) (*DocumentUBL, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/documents/"+documentID+"/ubl", nil)
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
		var ubl DocumentUBL
		if err := json.Unmarshal(body, &ubl); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &ubl, nil
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

// postMultipartFile sends a file as multipart/form-data POST request.
func (c *Client) postMultipartFile(url, filePath string) (*http.Response, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	filename := filepath.Base(filePath)
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copying file: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, "POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	return c.httpClient.Do(req)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+path, nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/lookup", nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/lookup/participants", nil)
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
	req, err := http.NewRequestWithContext(c.ctx, "GET", c.baseURL+"/api/validate/peppol-id", nil)
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

// ValidateJSON validates a JSON document file against Peppol BIS Billing 3.0.
func (c *Client) ValidateJSON(filePath string) (*ValidationResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	return c.validateJSONBody(data)
}

// ValidateJSONReader validates a JSON document from a reader against Peppol BIS Billing 3.0.
func (c *Client) ValidateJSONReader(r io.Reader) (*ValidationResponse, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	return c.validateJSONBody(data)
}

func (c *Client) validateJSONBody(data []byte) (*ValidationResponse, error) {
	req, err := http.NewRequestWithContext(c.ctx, "POST", c.baseURL+"/api/validate/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
	case http.StatusCreated, http.StatusOK:
		var val ValidationResponse
		if err := json.Unmarshal(body, &val); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &val, nil
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

// ValidateUBL validates a UBL/XML file against Peppol BIS Billing 3.0.
func (c *Client) ValidateUBL(filePath string) (*ValidationResponse, error) {
	resp, err := c.postMultipartFile(c.baseURL+"/api/validate/ubl", filePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		var val ValidationResponse
		if err := json.Unmarshal(body, &val); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &val, nil
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
	req2.Header.Set("User-Agent", "peppol-cli/"+version.Version)
	return t.base.RoundTrip(req2)
}

// verboseTransport logs HTTP method/URL/status/duration to a writer.
type verboseTransport struct {
	base http.RoundTripper
	w    io.Writer
}

func (t *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	fmt.Fprintf(t.w, ">> %s %s\n", req.Method, req.URL)
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(t.w, "<< error: %s (%s)\n", err, time.Since(start).Round(time.Millisecond))
		return nil, err
	}
	fmt.Fprintf(t.w, "<< %d %s (%s)\n", resp.StatusCode, http.StatusText(resp.StatusCode), time.Since(start).Round(time.Millisecond))
	return resp, nil
}
