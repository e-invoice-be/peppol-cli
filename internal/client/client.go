package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
