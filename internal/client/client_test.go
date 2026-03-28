package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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


func TestLookupPeppolID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lookup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("peppol_id") != "0208:1018265814" {
			t.Errorf("unexpected peppol_id: %s", r.URL.Query().Get("peppol_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"queryMetadata": {
				"identifierScheme": "iso6523-actorid-upis",
				"identifierValue": "0208:1018265814",
				"smlDomain": "edelivery.tech.ec.europa.eu",
				"timestamp": "2026-01-12T14:32:10.123456",
				"version": "1.0.0"
			},
			"status": "success",
			"dnsInfo": {
				"status": "success",
				"smpHostname": "smp.example.be",
				"smlHostname": "edelivery.tech.ec.europa.eu",
				"dnsRecords": []
			},
			"businessCard": {
				"status": "success",
				"entities": [
					{
						"name": "Example Corp",
						"countryCode": "BE",
						"identifiers": [{"scheme": "BE:CBE", "value": "1018265814"}]
					}
				],
				"queryTimeMs": 123.45
			},
			"executionTimeMs": 456.78
		}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.LookupPeppolID("0208:1018265814")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.QueryMetadata == nil || result.QueryMetadata.IdentifierValue != "0208:1018265814" {
		t.Error("expected queryMetadata with identifierValue '0208:1018265814'")
	}
	if result.BusinessCard == nil || len(result.BusinessCard.Entities) != 1 {
		t.Fatal("expected 1 business entity")
	}
	if *result.BusinessCard.Entities[0].Name != "Example Corp" {
		t.Errorf("expected entity name 'Example Corp', got %q", *result.BusinessCard.Entities[0].Name)
	}
}

func TestLookupPeppolID_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid Peppol ID format"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.LookupPeppolID("invalid")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("expected status 422, got %d", apiErr.StatusCode)
	}
	if apiErr.Detail != "Invalid Peppol ID format" {
		t.Errorf("expected detail 'Invalid Peppol ID format', got %q", apiErr.Detail)
	}
}

func TestLookupPeppolID_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.LookupPeppolID("0208:1018265814")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestSearchPeppolParticipants_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lookup/participants" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "test" {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("country_code") != "BE" {
			t.Errorf("unexpected country_code: %s", r.URL.Query().Get("country_code"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"total_count": 2,
			"used_count": 2,
			"participants": [
				{
					"peppol_id": "0208:1018265814",
					"peppol_scheme": "iso6523-actorid-upis",
					"entities": [{"name": "Test Corp", "country_code": "BE"}],
					"document_types": [{"scheme": "busdox-docid-qns", "value": "invoice"}]
				},
				{
					"peppol_id": "0208:0663934910",
					"peppol_scheme": "iso6523-actorid-upis",
					"entities": [{"name": "Another Corp", "country_code": "BE"}]
				}
			],
			"query_terms": "test",
			"search_date": "2026-03-27"
		}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.SearchPeppolParticipants("test", "BE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalCount != 2 {
		t.Errorf("expected total_count 2, got %d", result.TotalCount)
	}
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
	if result.Participants[0].PeppolID != "0208:1018265814" {
		t.Errorf("expected peppol_id '0208:1018265814', got %q", result.Participants[0].PeppolID)
	}
}

func TestSearchPeppolParticipants_NoCountry(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"total_count":0,"used_count":0,"participants":[],"query_terms":"test","search_date":"2026-03-27"}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.SearchPeppolParticipants("test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery.Get("country_code") != "" {
		t.Errorf("expected no country_code param, got %q", gotQuery.Get("country_code"))
	}
}

func TestValidatePeppolID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validate/peppol-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("peppol_id") != "0208:1018265814" {
			t.Errorf("unexpected peppol_id: %s", r.URL.Query().Get("peppol_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"is_valid": true,
			"dns_valid": true,
			"business_card_valid": true,
			"supported_document_types": ["invoice", "credit_note"],
			"business_card": {
				"name": "Example Corp",
				"country_code": "BE",
				"registration_date": "2021-06-15"
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.ValidatePeppolID("0208:1018265814")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Error("expected is_valid true")
	}
	if !result.DNSValid {
		t.Error("expected dns_valid true")
	}
	if !result.BusinessCardValid {
		t.Error("expected business_card_valid true")
	}
	if len(result.SupportedDocumentTypes) != 2 {
		t.Errorf("expected 2 document types, got %d", len(result.SupportedDocumentTypes))
	}
	if result.BusinessCard == nil || *result.BusinessCard.Name != "Example Corp" {
		t.Error("expected business card with name 'Example Corp'")
	}
}

func TestValidatePeppolID_ValidationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid Peppol ID format"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.ValidatePeppolID("bad-id")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("expected status 422, got %d", apiErr.StatusCode)
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

func TestListAttachments_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/doc-1/attachments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"att-1","file_name":"invoice.pdf","file_type":"application/pdf","file_size":1024}]`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	atts, err := c.ListAttachments("doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}
	if atts[0].ID != "att-1" {
		t.Errorf("expected id 'att-1', got %q", atts[0].ID)
	}
	if atts[0].FileName != "invoice.pdf" {
		t.Errorf("expected file_name 'invoice.pdf', got %q", atts[0].FileName)
	}
}

func TestListAttachments_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.ListAttachments("doc-1")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestGetAttachment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-1/attachments/att-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fileURL := "https://example.com/file.pdf"
		json.NewEncoder(w).Encode(DocumentAttachment{
			ID: "att-1", FileName: "invoice.pdf", FileType: "application/pdf", FileSize: 2048, FileURL: &fileURL,
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	att, err := c.GetAttachment("doc-1", "att-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if att.ID != "att-1" {
		t.Errorf("expected id 'att-1', got %q", att.ID)
	}
	if att.FileURL == nil || *att.FileURL != "https://example.com/file.pdf" {
		t.Errorf("unexpected file_url: %v", att.FileURL)
	}
}

func TestGetAttachment_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.GetAttachment("doc-1", "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAddAttachment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/doc-1/attachments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" || len(ct) < 10 {
			t.Error("expected multipart content-type")
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing file field: %v", err)
		}
		defer file.Close()
		if header.Filename != "test.pdf" {
			t.Errorf("expected filename 'test.pdf', got %q", header.Filename)
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"att-new","file_name":"test.pdf","file_type":"application/pdf","file_size":512}`))
	}))
	defer srv.Close()

	// Create a temp file to upload.
	tmp, err := os.CreateTemp(t.TempDir(), "test-*.pdf")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write([]byte("fake pdf content"))
	tmp.Close()
	// Rename to test.pdf for predictable filename.
	testFile := filepath.Join(t.TempDir(), "test.pdf")
	os.Rename(tmp.Name(), testFile)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	att, err := c.AddAttachment("doc-1", testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if att.ID != "att-new" {
		t.Errorf("expected id 'att-new', got %q", att.ID)
	}
	if att.FileName != "test.pdf" {
		t.Errorf("expected file_name 'test.pdf', got %q", att.FileName)
	}
}

func TestDeleteAttachment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/doc-1/attachments/att-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"is_deleted":true}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.DeleteAttachment("doc-1", "att-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsDeleted {
		t.Error("expected is_deleted to be true")
	}
}

// --- Document create/send/validate tests (from PRD-217) ---

func TestCreateDocumentJSON_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json content-type, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "INVOICE") {
			t.Errorf("expected body to contain INVOICE, got %s", string(body))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(DocumentResponse{ID: "doc-new", DocumentType: DocumentTypeInvoice})
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "invoice.json")
	os.WriteFile(tmpFile, []byte(`{"document_type":"INVOICE","items":[{"description":"Test"}]}`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.CreateDocumentJSON(tmpFile, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "doc-new" {
		t.Errorf("expected doc ID 'doc-new', got %q", doc.ID)
	}
}

func TestCreateDocumentJSON_ConstructPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("construct_pdf") != "true" {
			t.Errorf("expected construct_pdf=true query param")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(DocumentResponse{ID: "doc-pdf"})
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "invoice.json")
	os.WriteFile(tmpFile, []byte(`{}`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.CreateDocumentJSON(tmpFile, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "doc-pdf" {
		t.Errorf("expected doc ID 'doc-pdf', got %q", doc.ID)
	}
}

func TestCreateDocumentJSON_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "invoice.json")
	os.WriteFile(tmpFile, []byte(`{}`), 0644)

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.CreateDocumentJSON(tmpFile, false)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateDocumentFromUBL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/ubl" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart content-type, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(DocumentResponse{ID: "doc-ubl"})
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "invoice.xml")
	os.WriteFile(tmpFile, []byte(`<Invoice></Invoice>`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.CreateDocumentFromUBL(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "doc-ubl" {
		t.Errorf("expected doc ID 'doc-ubl', got %q", doc.ID)
	}
}

func TestCreateDocumentFromPDF_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("vendor_tax_id") != "BE123" {
			t.Errorf("expected vendor_tax_id=BE123")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(DocumentCreateFromPdfResponse{
			DocumentResponse: DocumentResponse{ID: "doc-pdf"},
			Success:          true,
		})
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "invoice.pdf")
	os.WriteFile(tmpFile, []byte(`%PDF-fake`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.CreateDocumentFromPDF(tmpFile, "BE123", "BE456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !doc.Success {
		t.Error("expected success=true")
	}
}

func TestSendDocument_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-123/send" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("sender_peppol_id") != "0088:sender" {
			t.Errorf("expected sender_peppol_id=0088:sender")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DocumentResponse{ID: "doc-123", State: DocumentStateSent})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	doc, err := c.SendDocument("doc-123", SendDocumentOptions{SenderPeppolID: "0088:sender"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.State != DocumentStateSent {
		t.Errorf("expected state SENT, got %q", doc.State)
	}
}

func TestSendDocument_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.SendDocument("nonexistent", SendDocumentOptions{})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestValidateDocument_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-123/validate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ValidationResponse{ID: "doc-123", IsValid: true, Issues: []ValidationIssue{}})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val.IsValid {
		t.Error("expected is_valid=true")
	}
}

func TestValidateDocument_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ValidationResponse{
			ID: "doc-123", IsValid: false,
			Issues: []ValidationIssue{{Message: "Missing buyer name", Type: IssueTypeError, Schematron: "BR-07"}},
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.IsValid {
		t.Error("expected is_valid=false")
	}
	if val.Issues[0].Schematron != "BR-07" {
		t.Errorf("expected schematron 'BR-07', got %q", val.Issues[0].Schematron)
	}
}

func TestDeleteDocument_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/documents/doc-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DocumentDelete{IsDeleted: true})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	result, err := c.DeleteDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsDeleted {
		t.Error("expected is_deleted=true")
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Document not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.DeleteDocument("doc-999")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteDocument_BadState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Document not in draft state"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.DeleteDocument("doc-123")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Detail != "Document not in draft state" {
		t.Errorf("expected detail about draft state, got %q", apiErr.Detail)
	}
}

func TestGetDocumentUBL_Success(t *testing.T) {
	signedURL := "https://storage.example.com/ubl.xml"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents/doc-123/ubl" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DocumentUBL{
			FileName:  "invoice.xml",
			FileSize:  4096,
			SignedURL: &signedURL,
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	ubl, err := c.GetDocumentUBL("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ubl.FileName != "invoice.xml" {
		t.Errorf("expected file_name 'invoice.xml', got %q", ubl.FileName)
	}
	if ubl.FileSize != 4096 {
		t.Errorf("expected file_size 4096, got %d", ubl.FileSize)
	}
	if ubl.SignedURL == nil || *ubl.SignedURL != signedURL {
		t.Error("expected signed_url to match")
	}
}

func TestGetDocumentUBL_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Document not found"})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.GetDocumentUBL("doc-999")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestValidateJSON_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validate/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ValidationResponse{ID: "val-1", IsValid: true, Issues: []ValidationIssue{}})
	}))
	defer srv.Close()

	// Create a temp JSON file.
	tmpFile := t.TempDir() + "/invoice.json"
	os.WriteFile(tmpFile, []byte(`{"invoice_id": "INV-001"}`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateJSON(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val.IsValid {
		t.Error("expected is_valid=true")
	}
}

func TestValidateJSON_Invalid(t *testing.T) {
	ruleID := "BR-07"
	location := "/Invoice/cac:AccountingCustomerParty"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ValidationResponse{
			ID: "val-2", IsValid: false,
			Issues: []ValidationIssue{
				{Message: "Missing buyer name", Type: IssueTypeError, RuleID: &ruleID, Location: &location, Schematron: "BR-07"},
				{Message: "Recommended field missing", Type: IssueTypeWarning, Schematron: "BR-CL-01"},
			},
		})
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/invalid.json"
	os.WriteFile(tmpFile, []byte(`{}`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateJSON(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.IsValid {
		t.Error("expected is_valid=false")
	}
	if len(val.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(val.Issues))
	}
	if val.Issues[0].Type != IssueTypeError {
		t.Errorf("expected first issue type 'error', got %q", val.Issues[0].Type)
	}
	if val.Issues[1].Type != IssueTypeWarning {
		t.Errorf("expected second issue type 'warning', got %q", val.Issues[1].Type)
	}
}

func TestValidateJSONReader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":true}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ValidationResponse{ID: "val-3", IsValid: true, Issues: []ValidationIssue{}})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateJSONReader(strings.NewReader(`{"test":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val.IsValid {
		t.Error("expected is_valid=true")
	}
}

func TestValidateUBL_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/validate/ubl" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart/form-data Content-Type, got %s", ct)
		}
		// Verify file is present in the multipart form.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("expected file field: %v", err)
		}
		defer file.Close()
		if header.Filename != "invoice.xml" {
			t.Errorf("expected filename 'invoice.xml', got %q", header.Filename)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ValidationResponse{ID: "val-4", IsValid: true, Issues: []ValidationIssue{}})
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/invoice.xml"
	os.WriteFile(tmpFile, []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"></Invoice>`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	val, err := c.ValidateUBL(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val.IsValid {
		t.Error("expected is_valid=true")
	}
}

func TestValidateUBL_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Detail: "Invalid API key"})
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/invoice.xml"
	os.WriteFile(tmpFile, []byte(`<Invoice/>`), 0644)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.ValidateUBL(tmpFile)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}
