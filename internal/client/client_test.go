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

// --- Document detail tests (from PRD-215) ---

func TestGetDocument_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "doc-123",
			"created_at": "2026-01-15T10:30:00Z",
			"document_type": "INVOICE",
			"state": "SENT",
			"direction": "OUTBOUND",
			"customer_name": "Acme Corp",
			"customer_tax_id": "BE0123456789",
			"vendor_name": "My Company",
			"invoice_id": "INV-001",
			"invoice_date": "2026-01-15",
			"due_date": "2026-02-15",
			"currency": "EUR",
			"subtotal": "1000.00",
			"total_tax": "210.00",
			"invoice_total": "1210.00",
			"amount_due": "1210.00",
			"payment_details": [{"iban": "BE71096123456769", "payment_reference": "+++123/4567/89012+++"}]
		}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.GetDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "doc-123" {
		t.Errorf("expected id 'doc-123', got %q", doc.ID)
	}
	if doc.DocumentType != DocumentTypeInvoice {
		t.Errorf("expected type INVOICE, got %q", doc.DocumentType)
	}
	if doc.State != DocumentStateSent {
		t.Errorf("expected state SENT, got %q", doc.State)
	}
	if *doc.CustomerName != "Acme Corp" {
		t.Errorf("expected customer 'Acme Corp', got %q", *doc.CustomerName)
	}
	if *doc.Subtotal != "1000.00" {
		t.Errorf("expected subtotal '1000.00', got %q", *doc.Subtotal)
	}
	if len(doc.PaymentDetails) != 1 || *doc.PaymentDetails[0].IBAN != "BE71096123456769" {
		t.Errorf("unexpected payment details: %+v", doc.PaymentDetails)
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.GetDocument("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetDocument_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.GetDocument("doc-123")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetDocumentTimeline_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-123/timeline" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"document_id": "doc-123",
			"events": [
				{"event_type": "document_created", "timestamp": "2026-01-15T10:30:00Z"},
				{"event_type": "send_success", "timestamp": "2026-01-15T10:31:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	timeline, err := c.GetDocumentTimeline("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timeline.DocumentID != "doc-123" {
		t.Errorf("expected document_id 'doc-123', got %q", timeline.DocumentID)
	}
	if len(timeline.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(timeline.Events))
	}
	if timeline.Events[0].EventType != TimelineDocumentCreated {
		t.Errorf("expected event_type 'document_created', got %q", timeline.Events[0].EventType)
	}
	if timeline.Events[1].EventType != TimelineSendSuccess {
		t.Errorf("expected event_type 'send_success', got %q", timeline.Events[1].EventType)
	}
}

func TestGetDocumentTimeline_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.GetDocumentTimeline("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Document list endpoint tests (from PRD-214) ---

func newDocumentListServer(t *testing.T, expectedPath string) *httptest.Server {
	t.Helper()
	invoiceID := "INV-001"
	vendorName := "Seller Co"
	customerName := "Buyer Co"
	total := "1234.56"
	date := "2026-01-15"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaginatedDocuments{
			Page: 1, PageSize: 20, Total: 1, Pages: 1, HasNextPage: false,
			Items: []DocumentResponse{{
				ID:           "doc-1",
				DocumentType: DocumentTypeInvoice,
				State:        DocumentStateReceived,
				InvoiceID:    &invoiceID,
				VendorName:   &vendorName,
				CustomerName: &customerName,
				InvoiceTotal: &total,
				InvoiceDate:  &date,
				Currency:     "EUR",
			}},
		})
	}))
}

func TestListInbox_Success(t *testing.T) {
	srv := newDocumentListServer(t, "/api/inbox/")
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListInbox(DocumentListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
	if result.Items[0].ID != "doc-1" {
		t.Errorf("expected doc-1, got %s", result.Items[0].ID)
	}
	if *result.Items[0].InvoiceTotal != "1234.56" {
		t.Errorf("expected 1234.56, got %s", *result.Items[0].InvoiceTotal)
	}
}

func TestListInbox_WithFilters(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaginatedDocuments{Page: 2, PageSize: 5, Total: 10, Pages: 2})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.ListInbox(DocumentListParams{
		Type:     "invoice",
		Sender:   "sender-123",
		FromDate: "2026-01-01",
		ToDate:   "2026-03-01",
		Search:   "test",
		Page:     2,
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q, _ := url.ParseQuery(gotQuery)
	checks := map[string]string{
		"type":      "invoice",
		"sender":    "sender-123",
		"date_from": "2026-01-01",
		"date_to":   "2026-03-01",
		"search":    "test",
		"page":      "2",
		"page_size": "5",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query param %s: got %q, want %q", key, got, want)
		}
	}
}

func TestListInbox_Unauthorized(t *testing.T) {
	srv := newDocumentListServer(t, "/api/inbox/")
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.ListInbox(DocumentListParams{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestListInboxInvoices_Success(t *testing.T) {
	srv := newDocumentListServer(t, "/api/inbox/invoices")
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListInboxInvoices(DocumentListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestListInboxCreditNotes_Success(t *testing.T) {
	srv := newDocumentListServer(t, "/api/inbox/credit-notes")
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListInboxCreditNotes(DocumentListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestListOutbox_Success(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/outbox/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaginatedDocuments{Page: 1, PageSize: 20, Total: 2, Pages: 1})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListOutbox(DocumentListParams{Receiver: "recv-456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("receiver") != "recv-456" {
		t.Errorf("expected receiver 'recv-456', got %q", q.Get("receiver"))
	}
}

func TestListOutboxDrafts_Success(t *testing.T) {
	srv := newDocumentListServer(t, "/api/outbox/drafts")
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListOutboxDrafts(DocumentListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Total)
	}
}

func TestListDrafts_Success(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/drafts/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PaginatedDocuments{Page: 1, PageSize: 20, Total: 3, Pages: 1})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ListDrafts(DocumentListParams{State: "DRAFT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("state") != "DRAFT" {
		t.Errorf("expected state 'DRAFT', got %q", q.Get("state"))
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
