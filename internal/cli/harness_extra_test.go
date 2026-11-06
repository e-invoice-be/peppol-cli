package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/e-invoice-be/peppol-cli/internal/config"
)

// The CLI commands build their API client with client.NewClient and the
// default base URL, so there is no option to point them at a test server.
// The client's transport wraps http.DefaultTransport, which these helpers
// swap for a transport that sends requests for the production API host to an
// httptest server and refuses every other non-loopback host. No test in this
// package ever reaches the network.

const (
	goodKey = "good-key"
	prodAPI = "api.e-invoice.be"
)

// recordedRequest is a request seen by the fake API.
type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Auth   string
	Body   []byte
	Header http.Header
}

// fakeAPI is an httptest server with routes keyed by "METHOD /path".
type fakeAPI struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	reqs   []recordedRequest
}

// redirectTransport sends production API requests to the fake server.
type redirectTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.URL.Host == prodAPI:
		req2 := req.Clone(req.Context())
		req2.URL.Scheme = rt.target.Scheme
		req2.URL.Host = rt.target.Host
		req2.Host = rt.target.Host
		return rt.base.RoundTrip(req2)
	case strings.HasPrefix(req.URL.Host, "127.0.0.1:"):
		return rt.base.RoundTrip(req)
	default:
		return nil, fmt.Errorf("test transport: refusing request to %s", req.URL.Host)
	}
}

// newFakeAPI starts a fake API and routes the production API host to it for
// the rest of the test. Requests must carry "Bearer good-key", otherwise the
// fake answers 401 like the real API does.
func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, routes: make(map[string]http.HandlerFunc)}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)

	target, err := url.Parse(f.srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	orig := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: target, base: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// The client went away mid-request (e.g. backup cancels in-flight
		// fetches after a sibling fails). Nothing to record or answer.
		return
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, recordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.Query(),
		Auth:   r.Header.Get("Authorization"),
		Body:   body,
		Header: r.Header.Clone(),
	})
	h, ok := f.routes[r.Method+" "+r.URL.Path]
	f.mu.Unlock()

	if strings.HasPrefix(r.URL.Path, "/files/") {
		// Signed download URLs carry no API key.
		if !ok {
			http.NotFound(w, r)
			return
		}
		h(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+goodKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Invalid API key"})
		return
	}
	if !ok {
		f.t.Errorf("fake API: unexpected request %s %s", r.Method, r.URL.Path)
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "no route"})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	h(w, r)
}

// handle registers a route ("GET /api/me/").
func (f *fakeAPI) handle(route string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = h
}

// json registers a route that answers status with v encoded as JSON.
func (f *fakeAPI) json(route string, status int, v any) {
	f.handle(route, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, status, v) })
}

// fileURL returns an absolute URL on the fake server under /files/.
func (f *fakeAPI) fileURL(name string) string { return f.srv.URL + "/files/" + name }

// requests returns a copy of the requests seen so far.
func (f *fakeAPI) requests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.reqs...)
}

// last returns the last request to path, failing the test if there is none.
func (f *fakeAPI) last(path string) recordedRequest {
	f.t.Helper()
	reqs := f.requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Path == path {
			return reqs[i]
		}
	}
	f.t.Fatalf("no request to %s; got %v", path, reqs)
	return recordedRequest{}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// isolateConfig points the config dir, HOME and the API key env var at a
// fresh temp dir and returns the peppol-cli config dir.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")
	// Global flags (e.g. --workspace from a previous test's CLI run) persist
	// across tests; start clean and restore afterwards.
	saved := flags
	flags = GlobalFlags{}
	t.Cleanup(func() { flags = saved })
	return filepath.Join(dir, "peppol-cli")
}

// useEnvKey isolates the config and authenticates through PEPPOL_API_KEY.
func useEnvKey(t *testing.T, key string) {
	t.Helper()
	isolateConfig(t)
	t.Setenv("PEPPOL_API_KEY", key)
}

// cliResult holds the outcome of a CLI invocation.
type cliResult struct {
	Out string
	Err error
}

// runCLI runs the root command with args and stdin, capturing its output.
func runCLI(t *testing.T, stdin string, args ...string) cliResult {
	t.Helper()
	cmd := NewRootCmd()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return cliResult{Out: out.String(), Err: err}
}

// exitCode returns the ExitError code of err, 0 for nil, and -1 for errors
// that are not ExitErrors (Execute maps those to 1).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return -1
}

// wantCode fails the test unless res failed with the given exit code.
func wantCode(t *testing.T, res cliResult, code int) {
	t.Helper()
	if got := exitCode(res.Err); got != code {
		t.Fatalf("exit code = %d, want %d (err: %v)\noutput:\n%s", got, code, res.Err, res.Out)
	}
}

// wantOK fails the test if res failed.
func wantOK(t *testing.T, res cliResult) {
	t.Helper()
	if res.Err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", res.Err, res.Out)
	}
}

// wantContains fails the test unless s contains every want.
func wantContains(t *testing.T, s string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("output missing %q\noutput:\n%s", w, s)
		}
	}
}

// decodeJSON decodes s into a map, failing the test if s is not one JSON object.
func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("output is not a JSON object: %v\noutput:\n%s", err, s)
	}
	return m
}

// writeTempFile writes content to name in a temp dir and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// saveWorkspaces writes a config with the given workspaces (name → tenant)
// and stores key for each into its workspace credentials file.
func saveWorkspaces(t *testing.T, dir, active string, keys map[string]string) {
	t.Helper()
	cfg := &config.Config{ActiveWorkspace: active, Workspaces: map[string]config.Workspace{}}
	for name, key := range keys {
		cfg.Workspaces[name] = config.Workspace{Name: "Tenant " + name}
		if key != "" {
			if err := config.NewFileKeyringForWorkspace(dir, name).Set(key); err != nil {
				t.Fatalf("store key for %s: %v", name, err)
			}
		}
	}
	if err := config.SaveTo(dir, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

// sampleDoc is a minimal document JSON as the API returns it.
func sampleDoc(id string) map[string]any {
	return map[string]any{
		"id":            id,
		"created_at":    "2026-01-15T10:00:00Z",
		"document_type": "INVOICE",
		"state":         "DRAFT",
		"direction":     "OUTBOUND",
		"invoice_id":    "INV-" + id,
		"customer_name": "Buyer BV",
		"vendor_name":   "Seller NV",
		"invoice_total": "121.00",
		"subtotal":      "100.00",
		"currency":      "EUR",
		"invoice_date":  "2026-01-15",
		"items": []map[string]any{
			{"description": "Widget", "quantity": "2", "unit_price": "50.00", "amount": "100.00"},
		},
	}
}

// docPage wraps docs in a paginated document list.
func docPage(docs ...map[string]any) map[string]any {
	return map[string]any{"items": docs, "total": len(docs), "page": 1, "page_size": 20, "pages": 1}
}
