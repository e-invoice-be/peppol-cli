package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGetMe_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TenantPublic{
			Name: "Test Tenant",
			Plan: "pro",
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	tenant, err := c.GetMe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant.Name != "Test Tenant" {
		t.Errorf("expected name 'Test Tenant', got %q", tenant.Name)
	}
	if tenant.Plan != "pro" {
		t.Errorf("expected plan 'pro', got %q", tenant.Plan)
	}
}

func TestGetMe_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.GetMe()
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetMe_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "something broke"})
	}))
	defer srv.Close()

	c := NewClient("key", WithBaseURL(srv.URL))
	_, err := c.GetMe()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
	if apiErr.Detail != "something broke" {
		t.Errorf("expected detail 'something broke', got %q", apiErr.Detail)
	}
}

func TestAuthTransport_InjectsBearer(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"test"}`))
	}))
	defer srv.Close()

	c := NewClient("my-secret-key", WithBaseURL(srv.URL))
	c.GetMe()
	if gotHeader != "Bearer my-secret-key" {
		t.Errorf("expected 'Bearer my-secret-key', got %q", gotHeader)
	}
}

func TestGetStats_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatsResponse{
			TenantID:          "t-123",
			PeriodStart:       "2026-01-01",
			PeriodEnd:         "2026-03-27",
			Aggregation:       StatsAggregationDay,
			TotalDays:         86,
			AverageDailyUsage: 3.5,
			Actions: []ActionStats{
				{Action: ActionDocumentSent, StatDate: "2026-01-01", Count: 5},
				{Action: ActionDocumentReceived, StatDate: "2026-01-01", Count: 3},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	stats, err := c.GetStats("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TenantID != "t-123" {
		t.Errorf("expected tenant_id 't-123', got %q", stats.TenantID)
	}
	if stats.TotalDays != 86 {
		t.Errorf("expected total_days 86, got %d", stats.TotalDays)
	}
	if len(stats.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(stats.Actions))
	}
}

func TestGetStats_WithFilters(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatsResponse{
			TenantID:    "t-123",
			PeriodStart: "2026-01-01",
			PeriodEnd:   "2026-03-01",
			Aggregation: StatsAggregationMonth,
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.GetStats("2026-01-01", "2026-03-01", "MONTH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q, _ := url.ParseQuery(gotQuery)
	if q.Get("start_date") != "2026-01-01" {
		t.Errorf("expected start_date '2026-01-01', got %q", q.Get("start_date"))
	}
	if q.Get("end_date") != "2026-03-01" {
		t.Errorf("expected end_date '2026-03-01', got %q", q.Get("end_date"))
	}
	if q.Get("aggregation") != "MONTH" {
		t.Errorf("expected aggregation 'MONTH', got %q", q.Get("aggregation"))
	}
}

func TestGetStats_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.GetStats("", "", "")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "****"},
		{"abc", "****"},
		{"abcd", "****"},
		{"abcde", "****...bcde"},
		{"sk-1234567890abcdef", "****...cdef"},
	}
	for _, tt := range tests {
		got := MaskKey(tt.input)
		if got != tt.want {
			t.Errorf("MaskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
