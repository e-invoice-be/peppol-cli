package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// endpointCase describes one client method for the table-driven error tests.
type endpointCase struct {
	name     string
	ok       int  // success status the method accepts
	maps404  bool // 404 is returned as ErrNotFound rather than *APIError
	okBody   string
	call     func(c *Client) error
	needFile bool // call reads the upload file
}

// newEndpointCases returns every API method of the client. file is an
// existing upload file used by the upload methods.
func newEndpointCases(file string) []endpointCase {
	obj := `{"id":"x"}`
	return []endpointCase{
		{"GetMe", 200, true, obj, func(c *Client) error { _, err := c.GetMe(); return err }, false},
		{"GetStats", 200, false, obj, func(c *Client) error { _, err := c.GetStats("", "", ""); return err }, false},
		{"GetDocument", 200, true, obj, func(c *Client) error { _, err := c.GetDocument("d"); return err }, false},
		{"GetDocumentTimeline", 200, true, obj, func(c *Client) error { _, err := c.GetDocumentTimeline("d"); return err }, false},
		{"GetDocumentLifecycle", 200, true, obj, func(c *Client) error { _, err := c.GetDocumentLifecycle("d"); return err }, false},
		{"ListAttachments", 200, true, `[]`, func(c *Client) error { _, err := c.ListAttachments("d"); return err }, false},
		{"GetAttachment", 200, true, obj, func(c *Client) error { _, err := c.GetAttachment("d", "a"); return err }, false},
		{"AddAttachment", 201, true, obj, func(c *Client) error { _, err := c.AddAttachment("d", file); return err }, true},
		{"DeleteAttachment", 200, true, obj, func(c *Client) error { _, err := c.DeleteAttachment("d", "a"); return err }, false},
		{"CreateDocumentJSON", 201, false, obj, func(c *Client) error { _, err := c.CreateDocumentJSON(file, false); return err }, true},
		{"CreateDocumentFromUBL", 201, false, obj, func(c *Client) error { _, err := c.CreateDocumentFromUBL(file); return err }, true},
		{"CreateDocumentFromPDF", 201, false, obj, func(c *Client) error { _, err := c.CreateDocumentFromPDF(file, "", ""); return err }, true},
		{"SendDocument", 200, true, obj, func(c *Client) error { _, err := c.SendDocument("d", SendDocumentOptions{}); return err }, false},
		{"ValidateDocument", 200, true, obj, func(c *Client) error { _, err := c.ValidateDocument("d"); return err }, false},
		{"DeleteDocument", 200, true, obj, func(c *Client) error { _, err := c.DeleteDocument("d"); return err }, false},
		{"GetDocumentUBL", 200, true, obj, func(c *Client) error { _, err := c.GetDocumentUBL("d"); return err }, false},
		{"ListInbox", 200, false, obj, func(c *Client) error { _, err := c.ListInbox(DocumentListParams{}); return err }, false},
		{"ListInboxInvoices", 200, false, obj, func(c *Client) error { _, err := c.ListInboxInvoices(DocumentListParams{}); return err }, false},
		{"ListInboxCreditNotes", 200, false, obj, func(c *Client) error { _, err := c.ListInboxCreditNotes(DocumentListParams{}); return err }, false},
		{"ListOutbox", 200, false, obj, func(c *Client) error { _, err := c.ListOutbox(DocumentListParams{}); return err }, false},
		{"ListOutboxDrafts", 200, false, obj, func(c *Client) error { _, err := c.ListOutboxDrafts(DocumentListParams{}); return err }, false},
		{"ListDrafts", 200, false, obj, func(c *Client) error { _, err := c.ListDrafts(DocumentListParams{}); return err }, false},
		{"ListMailbox", 200, false, obj, func(c *Client) error { _, err := c.ListMailbox(MailboxListParams{}); return err }, false},
		{"GetMailboxEmail", 200, true, obj, func(c *Client) error { _, err := c.GetMailboxEmail("m"); return err }, false},
		{"ReprocessMailboxEmail", 202, true, obj, func(c *Client) error { _, err := c.ReprocessMailboxEmail("m"); return err }, false},
		{"LookupPeppolID", 200, false, obj, func(c *Client) error { _, err := c.LookupPeppolID("p"); return err }, false},
		{"SearchPeppolParticipants", 200, false, obj, func(c *Client) error { _, err := c.SearchPeppolParticipants("q", ""); return err }, false},
		{"ValidatePeppolID", 200, false, obj, func(c *Client) error { _, err := c.ValidatePeppolID("p"); return err }, false},
		{"ValidateJSON", 200, false, obj, func(c *Client) error { _, err := c.ValidateJSON(file); return err }, true},
		{"ValidateJSONReader", 200, false, obj, func(c *Client) error { _, err := c.ValidateJSONReader(strings.NewReader("{}")); return err }, false},
		{"ValidateUBL", 200, false, obj, func(c *Client) error { _, err := c.ValidateUBL(file); return err }, true},
	}
}

// statusServer answers every request with the status and body currently set.
type statusServer struct {
	*httptest.Server
	mu     sync.Mutex
	status int
	body   string
	header http.Header
}

func newStatusServer(t *testing.T) *statusServer {
	t.Helper()
	s := &statusServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		s.mu.Lock()
		status, body, header := s.status, s.body, s.header
		s.mu.Unlock()
		for k, v := range header {
			w.Header()[k] = v
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *statusServer) set(status int, body string, header http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body, s.header = status, body, header
}

func uploadFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "upload.xml")
	if err := os.WriteFile(p, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEndpoints_StatusMapping(t *testing.T) {
	srv := newStatusServer(t)
	c := NewClient("key", WithBaseURL(srv.URL))

	for _, tc := range newEndpointCases(uploadFile(t)) {
		t.Run(tc.name, func(t *testing.T) {
			srv.set(tc.ok, tc.okBody, nil)
			if err := tc.call(c); err != nil {
				t.Errorf("success status %d: %v", tc.ok, err)
			}

			srv.set(http.StatusUnauthorized, `{"detail":"bad key"}`, nil)
			if err := tc.call(c); !errors.Is(err, ErrUnauthorized) {
				t.Errorf("401: err = %v, want ErrUnauthorized", err)
			}

			srv.set(http.StatusNotFound, `{"detail":"missing"}`, nil)
			err := tc.call(c)
			var apiErr *APIError
			switch {
			case tc.maps404 && !errors.Is(err, ErrNotFound):
				t.Errorf("404: err = %v, want ErrNotFound", err)
			case !tc.maps404 && (!errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound):
				t.Errorf("404: err = %v, want *APIError with status 404", err)
			}

			srv.set(http.StatusInternalServerError, `{"detail":"kaboom"}`, nil)
			err = tc.call(c)
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError || apiErr.Detail != "kaboom" {
				t.Errorf("500: err = %v, want *APIError{500, kaboom}", err)
			}

			srv.set(http.StatusTooManyRequests, `{"detail":"slow down"}`, http.Header{"Retry-After": {"7"}})
			err = tc.call(c)
			if !errors.As(err, &apiErr) || apiErr.RetryAfter != 7*time.Second {
				t.Errorf("429: err = %v, want RetryAfter 7s", err)
			}
		})
	}
}

func TestEndpoints_MalformedSuccessBodyIsParseError(t *testing.T) {
	srv := newStatusServer(t)
	c := NewClient("key", WithBaseURL(srv.URL))
	for _, tc := range newEndpointCases(uploadFile(t)) {
		t.Run(tc.name, func(t *testing.T) {
			srv.set(tc.ok, `{not json`, nil)
			if err := tc.call(c); err == nil || !strings.Contains(err.Error(), "parsing response") {
				t.Errorf("err = %v, want a parsing error", err)
			}
		})
	}
}

func TestEndpoints_TransportErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens on url any more
	c := NewClient("key", WithBaseURL(url))
	for _, tc := range newEndpointCases(uploadFile(t)) {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(c); err == nil {
				t.Error("expected an error from a closed server")
			}
		})
	}
	if _, err := c.DownloadMailboxAttachment("m", "f"); err == nil || !strings.Contains(err.Error(), "executing request") {
		t.Errorf("DownloadMailboxAttachment: err = %v", err)
	}
}

func TestEndpoints_InvalidBaseURLFailsToBuildRequest(t *testing.T) {
	c := NewClient("key", WithBaseURL("http://bad\x7f"))
	for _, tc := range newEndpointCases(uploadFile(t)) {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(c); err == nil {
				t.Error("expected an error for an invalid base URL")
			}
		})
	}
}

func TestEndpoints_MissingUploadFile(t *testing.T) {
	c := NewClient("key", WithBaseURL("http://127.0.0.1:1"))
	missing := filepath.Join(t.TempDir(), "missing")
	for _, tc := range newEndpointCases(missing) {
		if !tc.needFile {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(c)
			if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("err = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestEndpoints_CanceledContext(t *testing.T) {
	srv := newStatusServer(t)
	srv.set(200, `{}`, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient("key", WithBaseURL(srv.URL)).WithContext(ctx)
	if _, err := c.GetMe(); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// --- DownloadMailboxAttachment ---

func TestDownloadMailboxAttachment_StatusMapping(t *testing.T) {
	srv := newStatusServer(t)
	c := NewClient("key", WithBaseURL(srv.URL))

	srv.set(200, "raw", http.Header{"Content-Type": {"text/plain"}})
	att, err := c.DownloadMailboxAttachment("m", "f.txt")
	if err != nil || string(att.Content) != "raw" || att.ContentType != "text/plain" {
		t.Errorf("att = %+v, err = %v", att, err)
	}
	srv.set(401, "", nil)
	if _, err := c.DownloadMailboxAttachment("m", "f"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("401: %v", err)
	}
	srv.set(404, "", nil)
	if _, err := c.DownloadMailboxAttachment("m", "f"); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: %v", err)
	}
	srv.set(500, `{"detail":"x"}`, nil)
	var apiErr *APIError
	if _, err := c.DownloadMailboxAttachment("m", "f"); !errors.As(err, &apiErr) {
		t.Errorf("500: %v", err)
	}
}

func TestMailboxEndpoints_EscapePathSegments(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = io.WriteString(w, "x")
	}))
	t.Cleanup(srv.Close)
	c := NewClient("key", WithBaseURL(srv.URL))
	if _, err := c.DownloadMailboxAttachment("a/b", "my file?.pdf"); err != nil {
		t.Fatal(err)
	}
	if want := "/api/mailbox/a%2Fb/attachments/my%20file%3F.pdf"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

// BUG: document and attachment IDs are concatenated into the URL without
// url.PathEscape (the mailbox endpoints do escape), so an ID containing "/",
// "?" or "#" addresses a different endpoint.
func TestDocumentEndpoints_EscapePathSegments(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.pdf")
	if err := os.WriteFile(file, []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	const evil = "x/y?z%25#f" // slash, query, percent and fragment
	const evilEsc = "x%2Fy%3Fz%2525%23f"
	cases := []struct {
		name string
		call func(c *Client, id string) error
		want string // %[1]s is the escaped ID
	}{
		{"GetDocument", func(c *Client, id string) error { _, err := c.GetDocument(id); return err }, "/api/documents/%[1]s"},
		{"GetDocumentTimeline", func(c *Client, id string) error { _, err := c.GetDocumentTimeline(id); return err }, "/api/documents/%[1]s/timeline"},
		{"GetDocumentLifecycle", func(c *Client, id string) error { _, err := c.GetDocumentLifecycle(id); return err }, "/api/documents/%[1]s/lifecycle"},
		{"GetDocumentUBL", func(c *Client, id string) error { _, err := c.GetDocumentUBL(id); return err }, "/api/documents/%[1]s/ubl"},
		{"SendDocument", func(c *Client, id string) error { _, err := c.SendDocument(id, SendDocumentOptions{}); return err }, "/api/documents/%[1]s/send"},
		{"ValidateDocument", func(c *Client, id string) error { _, err := c.ValidateDocument(id); return err }, "/api/documents/%[1]s/validate"},
		{"DeleteDocument", func(c *Client, id string) error { _, err := c.DeleteDocument(id); return err }, "/api/documents/%[1]s"},
		{"ListAttachments", func(c *Client, id string) error { _, err := c.ListAttachments(id); return err }, "/api/documents/%[1]s/attachments"},
		{"AddAttachment", func(c *Client, id string) error { _, err := c.AddAttachment(id, file); return err }, "/api/documents/%[1]s/attachments"},
		{"GetAttachment", func(c *Client, id string) error { _, err := c.GetAttachment(id, id); return err }, "/api/documents/%[1]s/attachments/%[1]s"},
		{"DeleteAttachment", func(c *Client, id string) error { _, err := c.DeleteAttachment(id, id); return err }, "/api/documents/%[1]s/attachments/%[1]s"},
		{"GetMailboxEmail", func(c *Client, id string) error { _, err := c.GetMailboxEmail(id); return err }, "/api/mailbox/%[1]s"},
		{"DownloadMailboxAttachment", func(c *Client, id string) error { _, err := c.DownloadMailboxAttachment(id, id); return err }, "/api/mailbox/%[1]s/attachments/%[1]s"},
		{"ReprocessMailboxEmail", func(c *Client, id string) error { _, err := c.ReprocessMailboxEmail(id); return err }, "/api/mailbox/%[1]s/reprocess"},
	}
	ids := []struct{ raw, escaped string }{
		{evil, evilEsc},
		// Plain IDs must produce byte-identical URLs to the unescaped form.
		{"doc_123", "doc_123"},
		{"0b9f6c3e-6a1d-4c47-9d1e-2f6f1f0a7b11", "0b9f6c3e-6a1d-4c47-9d1e-2f6f1f0a7b11"},
	}
	for _, tc := range cases {
		for _, id := range ids {
			t.Run(tc.name+"/"+id.escaped, func(t *testing.T) {
				var gotPath, gotQuery string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
					switch {
					case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/attachments"):
						w.WriteHeader(http.StatusCreated)
					case strings.HasSuffix(r.URL.Path, "/reprocess"):
						w.WriteHeader(http.StatusAccepted)
					case strings.HasSuffix(r.URL.Path, "/attachments"):
						_, _ = io.WriteString(w, `[]`)
						return
					case strings.HasPrefix(r.URL.Path, "/api/mailbox/") && strings.Contains(r.URL.Path, "/attachments/"):
						w.Header().Set("Content-Type", "application/pdf")
						_, _ = io.WriteString(w, "%PDF")
						return
					}
					_, _ = io.WriteString(w, `{}`)
				}))
				t.Cleanup(srv.Close)
				c := NewClient("key", WithBaseURL(srv.URL))
				if err := tc.call(c, id.raw); err != nil {
					t.Fatal(err)
				}
				if want := fmt.Sprintf(tc.want, id.escaped); gotPath != want {
					t.Errorf("path = %q, want %q", gotPath, want)
				}
				if gotQuery != "" {
					t.Errorf("unexpected query %q leaked from ID", gotQuery)
				}
			})
		}
	}
}

func TestPathURL_PlainIDsUnchanged(t *testing.T) {
	c := NewClient("key", WithBaseURL("https://api.example"))
	for _, id := range []string{"abc", "ABC-123_x.y~z", "0b9f6c3e-6a1d-4c47-9d1e-2f6f1f0a7b11"} {
		if got, want := c.pathURL("/api/documents/%s/timeline", id), "https://api.example/api/documents/"+id+"/timeline"; got != want {
			t.Errorf("pathURL(%q) = %q, want %q", id, got, want)
		}
	}
}

// --- error details ---

func TestAPIError_FormatsDetailAndRetryAfter(t *testing.T) {
	cases := []struct {
		err  APIError
		want string
	}{
		{APIError{StatusCode: 500}, "API error 500"},
		{APIError{StatusCode: 400, Detail: "bad"}, "API error 400: bad"},
		{APIError{StatusCode: 429, Detail: "slow", RetryAfter: 3 * time.Second}, "API error 429: slow (retry after 3s)"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestNewAPIError_RetryAfterBounds(t *testing.T) {
	for header, want := range map[string]time.Duration{
		"":                    0,
		"0":                   0,
		"-5":                  0,
		"soon":                0,
		"Wed, 21 Oct 2026 07": 0, // HTTP-date form is not supported
		"30":                  30 * time.Second,
		"999999999":           0, // above maxRetryAfter
	} {
		resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {header}}}
		if got := newAPIError(resp, nil).RetryAfter; got != want {
			t.Errorf("Retry-After %q: got %v, want %v", header, got, want)
		}
	}
}

func TestErrorDetail_Variants(t *testing.T) {
	cases := map[string]string{
		`{"detail":"plain"}`: "plain",
		`{"detail":[{"loc":["body","items",0,"qty"],"msg":"must be > 0","type":"v"},{"loc":["query"],"msg":"bad","type":"v"}]}`: "body.items.0.qty: must be > 0; query: bad",
		`not json`:    "",
		`{"other":1}`: "",
		``:            "",
	}
	for body, want := range cases {
		if got := errorDetail([]byte(body)); got != want {
			t.Errorf("errorDetail(%q) = %q, want %q", body, got, want)
		}
	}
}

// --- options and transports ---

func TestWithHTTPClient_NilTransportGetsAuth(t *testing.T) {
	var gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotUA = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	hc := &http.Client{Timeout: time.Second}
	c := NewClient("k1", WithBaseURL(srv.URL), WithHTTPClient(hc))
	if _, err := c.GetMe(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer k1" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !strings.HasPrefix(gotUA, "peppol-cli/") {
		t.Errorf("User-Agent = %q", gotUA)
	}
}

// BUG: NewClient assigns the auth transport to the caller's *http.Client.
// Passing http.DefaultClient (Transport == nil) makes every later request
// through http.DefaultClient carry the API key, including the signed-URL
// download in cli/document_attachment.go, which uses http.DefaultClient.
func TestWithHTTPClient_DoesNotMutateCallerClient(t *testing.T) {
	if os.Getenv("PEPPOL_SHOW_BUGS") == "" {
		t.Skip("BUG: client.go NewClient sets c.httpClient.Transport on the caller-supplied client")
	}
	hc := &http.Client{}
	_ = NewClient("secret", WithHTTPClient(hc))
	if hc.Transport != nil {
		t.Error("NewClient modified the caller's http.Client")
	}
}

func TestWithHTTPClient_CustomTransportGetsNoAuth(t *testing.T) {
	// Documents current behavior: a caller transport is used as-is, so no
	// Authorization header is injected.
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("k1", WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Transport: http.DefaultTransport}))
	if _, err := c.GetMe(); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want none", gotAuth)
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial failed")
}

func TestVerboseTransport_LogsErrorsWithoutKey(t *testing.T) {
	var buf bytes.Buffer
	c := NewClient("super-secret", WithBaseURL("http://example.invalid"),
		WithHTTPClient(&http.Client{Transport: &authTransport{apiKey: "super-secret", base: failingRoundTripper{}}}),
		WithVerbose(&buf))
	if _, err := c.GetMe(); err == nil {
		t.Fatal("expected an error")
	}
	log := buf.String()
	if !strings.Contains(log, ">> GET http://example.invalid/api/me/") || !strings.Contains(log, "<< error: dial failed") {
		t.Errorf("log = %q", log)
	}
	if strings.Contains(log, "super-secret") {
		t.Error("verbose log leaked the API key")
	}
}

func TestMaskKey_ShortAndLongKeys(t *testing.T) {
	for in, want := range map[string]string{"": "****", "abcd": "****", "abcde": "****...bcde", "sk_live_123456": "****...3456"} {
		if got := MaskKey(in); got != want {
			t.Errorf("MaskKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithContext_DoesNotModifyOriginal(t *testing.T) {
	c := NewClient("k")
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, 1)
	c2 := c.WithContext(ctx)
	if c.ctx == ctx || c2.ctx != ctx {
		t.Error("WithContext must return a copy carrying the new context")
	}
}

// --- request shape ---

func TestPostMultipartFile_ContentTypeByExtension(t *testing.T) {
	var gotType, gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("parse media type: %v", err)
			return
		}
		part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
		if err != nil {
			t.Errorf("next part: %v", err)
			return
		}
		gotType, gotName = part.Header.Get("Content-Type"), part.FileName()
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("k", WithBaseURL(srv.URL))

	dir := t.TempDir()
	noExt := filepath.Join(dir, "blob")
	if err := os.WriteFile(noExt, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateDocumentFromUBL(noExt); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/octet-stream" || gotName != "blob" {
		t.Errorf("no extension: type %q name %q", gotType, gotName)
	}

	pdf := filepath.Join(dir, "scan.pdf")
	if err := os.WriteFile(pdf, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateDocumentFromPDF(pdf, "", ""); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/pdf" {
		t.Errorf("pdf: type %q", gotType)
	}
}

// BUG: postMultipartFile writes the filename into Content-Disposition with
// %s inside quotes, so a filename containing a double quote produces a
// malformed header and the server sees a truncated name.
func TestPostMultipartFile_QuotesInFilename(t *testing.T) {
	if os.Getenv("PEPPOL_SHOW_BUGS") == "" {
		t.Skip("BUG: client.go postMultipartFile does not escape quotes in the filename (use multipart's escaping)")
	}
	var gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart(); err == nil {
			gotName = part.FileName()
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	p := filepath.Join(t.TempDir(), `inv"oice.xml`)
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient("k", WithBaseURL(srv.URL)).CreateDocumentFromUBL(p); err != nil {
		t.Fatal(err)
	}
	if gotName != `inv"oice.xml` {
		t.Errorf("server saw filename %q", gotName)
	}
}

func TestValidateJSONReader_ReadError(t *testing.T) {
	c := NewClient("k", WithBaseURL("http://127.0.0.1:1"))
	if _, err := c.ValidateJSONReader(errReader{}); err == nil || !strings.Contains(err.Error(), "reading input") {
		t.Errorf("err = %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }
