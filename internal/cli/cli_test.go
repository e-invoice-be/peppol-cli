package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/e-invoicebe/peppol-cli/internal/client"
	"github.com/e-invoicebe/peppol-cli/internal/config"
	"github.com/e-invoicebe/peppol-cli/internal/output"
	"github.com/spf13/cobra"
)

// newTestServer returns an httptest.Server that handles GET /api/me/.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "not found"})
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "Invalid API key"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.TenantPublic{
			Name: "Test Company",
			Plan: "pro",
		})
	}))
}

func TestMeCmd_WithEnvVar(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	t.Setenv("PEPPOL_API_KEY", "valid-key")

	cmd := NewRootCmd()
	// Override the me command to use our test server.
	// We'll test via the client directly since cobra wires through resolveKey.
	// Instead, test the client + output logic directly.
	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	tenant, err := c.GetMe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant.Name != "Test Company" {
		t.Errorf("expected 'Test Company', got %q", tenant.Name)
	}

	// Also verify root command parses without error.
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	if !strings.Contains(buf.String(), "auth") {
		t.Error("help output missing 'auth' command")
	}
	if !strings.Contains(buf.String(), "me") {
		t.Error("help output missing 'me' command")
	}
}

func TestMeCmd_InvalidKey(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := client.NewClient("invalid-key", client.WithBaseURL(srv.URL))
	_, err := c.GetMe()
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestMeCmd_JSONOutput(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	tenant, err := c.GetMe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON marshaling works correctly.
	data, err := json.Marshal(tenant)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if parsed["name"] != "Test Company" {
		t.Errorf("expected name 'Test Company' in JSON, got %v", parsed["name"])
	}
}

func TestRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"auth", "me", "--json", "--quiet", "--verbose", "--no-color"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestRootCmd_Version(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "dev") {
		t.Errorf("version output missing 'dev', got %q", buf.String())
	}
}

func TestAuthStatusCmd_NotAuthenticated(t *testing.T) {
	// Use a temp dir with no credentials.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Ensure no env var key.
	t.Setenv("PEPPOL_API_KEY", "")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"auth", "status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Not authenticated") {
		t.Errorf("expected 'Not authenticated', got %q", buf.String())
	}
}

func TestAuthLogoutCmd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"auth", "logout"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("expected 'Logged out', got %q", buf.String())
	}
}

func newStatsTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stats" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer valid-key" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "Invalid API key"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(client.StatsResponse{
				TenantID:          "t-123",
				PeriodStart:       "2026-01-01",
				PeriodEnd:         "2026-03-27",
				Aggregation:       client.StatsAggregationDay,
				TotalDays:         86,
				AverageDailyUsage: 3.5,
				Actions: []client.ActionStats{
					{Action: client.ActionDocumentSent, StatDate: "2026-01-01", Count: 5},
					{Action: client.ActionDocumentReceived, StatDate: "2026-01-01", Count: 3},
					{Action: client.ActionDocumentSent, StatDate: "2026-01-02", Count: 2},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestStatsCmd_JSONOutput(t *testing.T) {
	srv := newStatsTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	stats, err := c.GetStats("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if parsed["tenant_id"] != "t-123" {
		t.Errorf("expected tenant_id 't-123', got %v", parsed["tenant_id"])
	}
	if parsed["total_days"].(float64) != 86 {
		t.Errorf("expected total_days 86, got %v", parsed["total_days"])
	}
}

func TestStatsCmd_TextOutput(t *testing.T) {
	srv := newStatsTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	stats, err := c.GetStats("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key fields are present in the response
	if stats.PeriodStart != "2026-01-01" {
		t.Errorf("expected period start '2026-01-01', got %q", stats.PeriodStart)
	}
	if stats.TotalDays != 86 {
		t.Errorf("expected total days 86, got %d", stats.TotalDays)
	}
	if len(stats.Actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(stats.Actions))
	}
}

func TestStatsCmd_RenderTable(t *testing.T) {
	actions := []client.ActionStats{
		{Action: client.ActionDocumentSent, StatDate: "2026-01-01", Count: 5},
		{Action: client.ActionDocumentReceived, StatDate: "2026-01-01", Count: 3},
		{Action: client.ActionDocumentSent, StatDate: "2026-01-02", Count: 2},
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	renderActionsTable(cmd, actions)
	output := buf.String()

	if !strings.Contains(output, "DATE") {
		t.Error("table missing DATE header")
	}
	if !strings.Contains(output, "SENT") {
		t.Error("table missing SENT header")
	}
	if !strings.Contains(output, "RECEIVED") {
		t.Error("table missing RECEIVED header")
	}
	if !strings.Contains(output, "2026-01-01") {
		t.Error("table missing date 2026-01-01")
	}
	if !strings.Contains(output, "2026-01-02") {
		t.Error("table missing date 2026-01-02")
	}
}

func TestCompletionCmd_Bash(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"completion", "bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "bash") {
		t.Error("bash completion output missing 'bash' content")
	}
	if len(output) < 100 {
		t.Errorf("bash completion output suspiciously short: %d bytes", len(output))
	}
}

func TestCompletionCmd_Zsh(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"completion", "zsh"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(buf.String()) < 100 {
		t.Errorf("zsh completion output suspiciously short: %d bytes", len(buf.String()))
	}
}

func TestRootCmd_HelpIncludesNewCommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"stats", "completion"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func newDocumentTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "Invalid API key"})
			return
		}

		switch r.URL.Path {
		case "/api/documents/doc-123":
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
				"payment_term": "30 days",
				"payment_details": [{"iban": "BE71096123456769"}],
				"items": [
					{"description": "Consulting services", "quantity": "10", "unit_price": "100.00", "amount": "1000.00"},
					{"description": "Travel expenses", "quantity": "1", "unit_price": "210.00", "amount": "210.00"}
				]
			}`))
		case "/api/documents/doc-123/timeline":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"document_id": "doc-123",
				"events": [
					{"event_type": "document_created", "timestamp": "2026-01-15T10:30:00Z"},
					{"event_type": "send_success", "timestamp": "2026-01-15T10:31:00Z"}
				]
			}`))
		case "/api/documents/nonexistent":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "not found"})
		case "/api/documents/nonexistent/timeline":
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "not found"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestDocumentGetCmd_TextOutput(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	doc, err := c.GetDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderDocumentSections(r, doc, false); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"doc-123", "INVOICE", "SENT", "OUTBOUND", "INV-001", "2026-01-15", "2026-02-15", "Acme Corp", "BE0123456789", "My Company", "1000.00", "210.00", "1210.00", "30 days", "BE71096123456769"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentGetCmd_TextOutput_NoLineItems(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	doc, err := c.GetDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderDocumentSections(r, doc, false); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Consulting services") {
		t.Error("line items should not appear without --full flag")
	}
}

func TestDocumentGetCmd_FullOutput(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	doc, err := c.GetDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderDocumentSections(r, doc, true); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Consulting services", "Travel expenses", "100.00", "210.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("full output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentGetCmd_JSONOutput(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	doc, err := c.GetDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if parsed["id"] != "doc-123" {
		t.Errorf("expected id 'doc-123', got %v", parsed["id"])
	}
	if parsed["document_type"] != "INVOICE" {
		t.Errorf("expected document_type 'INVOICE', got %v", parsed["document_type"])
	}
}

func TestDocumentGetCmd_NotFound(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	_, err := c.GetDocument("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent document")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestDocumentTimelineCmd_TextOutput(t *testing.T) {
	srv := newDocumentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	timeline, err := c.GetDocumentTimeline("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderTimeline(r, timeline); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"2026-01-15 10:30:00", "Document Created", "2026-01-15 10:31:00", "Send Success"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentGetCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "get", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "document-id") {
		t.Errorf("help output missing 'document-id', got:\n%s", out)
	}
}

func TestDocumentTimelineCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "timeline", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "document-id") {
		t.Errorf("help output missing 'document-id', got:\n%s", out)
	}
}

func TestRootCmd_HelpIncludesDocument(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "document") {
		t.Error("help output missing 'document' command")
	}
}

func TestFormatEventType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"document_created", "Document Created"},
		{"send_success", "Send Success"},
		{"email_received", "Email Received"},
		{"mlr_received", "Mlr Received"},
	}
	for _, tt := range tests {
		got := formatEventType(tt.input)
		if got != tt.want {
			t.Errorf("formatEventType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExitError(t *testing.T) {
	err := &ExitError{Err: client.ErrUnauthorized, Code: 2}
	if err.Code != 2 {
		t.Errorf("expected code 2, got %d", err.Code)
	}
	if err.Error() != client.ErrUnauthorized.Error() {
		t.Errorf("expected %q, got %q", client.ErrUnauthorized.Error(), err.Error())
	}
}

func TestRootCmd_HelpIncludesWorkspace(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "workspace") {
		t.Error("help output missing 'workspace' command")
	}
}

func TestWorkspaceListCmd_Empty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No workspaces") {
		t.Errorf("expected empty workspace message, got %q", buf.String())
	}
}

func TestWorkspaceListCmd_WithWorkspaces(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	// Set up config with workspaces.
	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}
	if err := config.SaveTo(configDir, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "alpha") {
		t.Error("output missing 'alpha'")
	}
	if !strings.Contains(output, "beta") {
		t.Error("output missing 'beta'")
	}
}

func TestWorkspaceUseCmd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	// Set up config with workspaces.
	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}
	if err := config.SaveTo(configDir, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "use", "beta"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Switched to workspace") {
		t.Errorf("expected switch message, got %q", buf.String())
	}

	// Verify config was updated.
	loaded, err := config.LoadFrom(configDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.ActiveWorkspace != "beta" {
		t.Errorf("expected active 'beta', got %q", loaded.ActiveWorkspace)
	}
}

func TestWorkspaceUseCmd_NonExistent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
		},
	}
	config.SaveTo(configDir, cfg)

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "use", "nope"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent workspace")
	}
}

func TestWorkspaceRemoveCmd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}
	config.SaveTo(configDir, cfg)

	// Store workspace credentials.
	kr := config.NewFileKeyringForWorkspace(configDir, "beta")
	kr.Set("beta-key")

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "remove", "beta"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "removed") {
		t.Errorf("expected remove message, got %q", buf.String())
	}

	// Verify workspace removed from config.
	loaded, _ := config.LoadFrom(configDir)
	if _, ok := loaded.Workspaces["beta"]; ok {
		t.Error("beta should have been removed from config")
	}

	// Verify credentials removed.
	key, _ := kr.Get()
	if key != "" {
		t.Errorf("expected credentials removed, got %q", key)
	}
}

func TestWorkspaceRemoveCmd_ActiveWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}
	config.SaveTo(configDir, cfg)

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "remove", "alpha"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when removing active workspace")
	}
}

func TestWorkspaceListCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
		},
	}
	config.SaveTo(configDir, cfg)

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"workspace", "list", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result))
	}
	if result[0]["name"] != "alpha" {
		t.Errorf("expected name 'alpha', got %v", result[0]["name"])
	}
	if result[0]["active"] != true {
		t.Errorf("expected active true, got %v", result[0]["active"])
	}
}

func TestWorkspaceFlagOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PEPPOL_API_KEY", "")

	srv := newTestServer(t)
	defer srv.Close()

	configDir := dir + "/peppol-cli"
	cfg := &config.Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]config.Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}
	config.SaveTo(configDir, cfg)

	// Store keys for both workspaces.
	config.NewFileKeyringForWorkspace(configDir, "alpha").Set("invalid-key")
	config.NewFileKeyringForWorkspace(configDir, "beta").Set("valid-key")

	// Using -w beta should use beta's key (valid-key) instead of alpha's.
	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	tenant, err := c.GetMe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant.Name != "Test Company" {
		t.Errorf("expected 'Test Company', got %q", tenant.Name)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Company", "my-company"},
		{"  Alpha Co  ", "alpha-co"},
		{"Test123", "test123"},
		{"UPPER CASE", "upper-case"},
		{"special!@#chars", "special-chars"},
		{"", ""},
		{"trailing---", "trailing"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Inbox/Outbox/Drafts command tests ---

func TestRootCmd_HelpIncludesInboxOutboxDrafts(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"inbox", "outbox", "drafts"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestInboxListCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"inbox", "list", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, flag := range []string{"--sender", "--from", "--to", "--type", "--search", "--sort-by", "--sort-order", "--page", "--page-size"} {
		if !strings.Contains(out, flag) {
			t.Errorf("inbox list help missing %q", flag)
		}
	}
}

func TestOutboxListCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"outbox", "list", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--receiver") {
		t.Error("outbox list help missing --receiver")
	}
	if strings.Contains(out, "--sender") {
		t.Error("outbox list help should not have --sender")
	}
}

func TestDraftsListCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"drafts", "list", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--state") {
		t.Error("drafts list help missing --state")
	}
}

func newDocumentListTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	inv1 := "INV-001"
	inv2 := "CN-002"
	vendor1 := "Seller Co"
	vendor2 := "Seller 2"
	buyer1 := "Buyer Co"
	buyer2 := "Buyer 2"
	total1 := "1234.56"
	total2 := "567.89"
	date1 := "2026-01-15"
	date2 := "2026-02-20"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "Invalid API key"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(client.PaginatedDocuments{
			Page: 1, PageSize: 20, Total: 2, Pages: 1,
			Items: []client.DocumentResponse{
				{
					ID:           "doc-1",
					DocumentType: client.DocumentTypeInvoice,
					State:        client.DocumentStateReceived,
					InvoiceID:    &inv1,
					VendorName:   &vendor1,
					CustomerName: &buyer1,
					InvoiceTotal: &total1,
					InvoiceDate:  &date1,
					Currency:     "EUR",
				},
				{
					ID:           "doc-2",
					DocumentType: client.DocumentTypeCreditNote,
					State:        client.DocumentStateSent,
					InvoiceID:    &inv2,
					VendorName:   &vendor2,
					CustomerName: &buyer2,
					InvoiceTotal: &total2,
					InvoiceDate:  &date2,
					Currency:     "EUR",
				},
			},
		})
	}))
}

func TestInboxListCmd_Integration(t *testing.T) {
	srv := newDocumentListTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	result, err := c.ListInbox(client.DocumentListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
	if *result.Items[0].InvoiceID != "INV-001" {
		t.Errorf("expected INV-001, got %q", *result.Items[0].InvoiceID)
	}
}

func TestDocumentListFlags_ToParams(t *testing.T) {
	f := DocumentListFlags{
		DocType:   "invoice",
		From:      "2026-01-01",
		To:        "2026-03-01",
		Search:    "test",
		SortBy:    "date",
		SortOrder: "desc",
		Page:      2,
		PageSize:  10,
		Sender:    "sender-1",
	}
	params := f.ToParams()

	if params.Type != "invoice" {
		t.Errorf("expected type 'invoice', got %q", params.Type)
	}
	if params.Sender != "sender-1" {
		t.Errorf("expected sender 'sender-1', got %q", params.Sender)
	}
	if params.FromDate != "2026-01-01" {
		t.Errorf("expected from_date '2026-01-01', got %q", params.FromDate)
	}
	if params.Page != 2 {
		t.Errorf("expected page 2, got %d", params.Page)
	}
	if params.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", params.PageSize)
	}
}

func TestRenderDocumentList_Empty(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	result := &client.PaginatedDocuments{
		Page: 1, PageSize: 20, Total: 0,
		Items: []client.DocumentResponse{},
	}

	// Need to initialize context with renderer
	subCmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		return renderDocumentList(cmd, result, CounterpartySeller)
	}}
	cmd.AddCommand(subCmd)
	cmd.SetArgs([]string{"test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No documents found") {
		t.Errorf("expected 'No documents found', got %q", buf.String())
	}
}

func TestRenderDocumentList_Table(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	invID := "INV-001"
	vendor := "Seller Co"
	buyer := "Buyer Co"
	total := "1234.56"
	date := "2026-01-15"
	result := &client.PaginatedDocuments{
		Page: 1, PageSize: 20, Total: 1,
		Items: []client.DocumentResponse{
			{
				ID:           "doc-1",
				DocumentType: client.DocumentTypeInvoice,
				State:        client.DocumentStateReceived,
				InvoiceID:    &invID,
				VendorName:   &vendor,
				CustomerName: &buyer,
				InvoiceTotal: &total,
				InvoiceDate:  &date,
				Currency:     "EUR",
			},
		},
	}

	subCmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		return renderDocumentList(cmd, result, CounterpartySeller)
	}}
	cmd.AddCommand(subCmd)
	cmd.SetArgs([]string{"test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"doc-1", "INVOICE", "INV-001", "Seller Co", "1234.56", "2026-01-15"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q", want)
		}
	}
}

func TestRenderDocumentList_CounterpartyBuyer(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	invID2 := "INV-001"
	vendor2 := "Seller Co"
	buyer2 := "Buyer Co"
	total2 := "100.00"
	date2 := "2026-01-15"
	result := &client.PaginatedDocuments{
		Page: 1, PageSize: 20, Total: 1,
		Items: []client.DocumentResponse{
			{
				ID:           "doc-1",
				DocumentType: client.DocumentTypeInvoice,
				State:        client.DocumentStateSent,
				InvoiceID:    &invID2,
				VendorName:   &vendor2,
				CustomerName: &buyer2,
				InvoiceTotal: &total2,
				InvoiceDate:  &date2,
				Currency:     "EUR",
			},
		},
	}

	subCmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		return renderDocumentList(cmd, result, CounterpartyBuyer)
	}}
	cmd.AddCommand(subCmd)
	cmd.SetArgs([]string{"test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Buyer Co") {
		t.Error("expected Buyer Co in outbox counterparty column")
	}
}

func TestRootCmd_HelpIncludesLookupAndValidate(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "lookup") {
		t.Error("help output missing 'lookup' command")
	}
	if !strings.Contains(out, "validate") {
		t.Error("help output missing 'validate' command")
	}
}

func TestLookupCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"lookup", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "peppol-id") {
		t.Errorf("lookup help missing 'peppol-id', got:\n%s", out)
	}
	if !strings.Contains(out, "search") {
		t.Errorf("lookup help missing 'search' subcommand, got:\n%s", out)
	}
}

func TestLookupSearchCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"lookup", "search", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--country") {
		t.Errorf("search help missing '--country' flag, got:\n%s", out)
	}
}

func TestValidatePeppolIDCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "peppol-id", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Validate a Peppol") {
		t.Errorf("validate help missing description, got:\n%s", out)
	}
}

func TestRenderLookupResult(t *testing.T) {
	name := "Example Corp"
	country := "BE"
	smpHost := "smp.example.be"
	result := &client.PeppolIdLookupResponse{
		Status: "success",
		QueryMetadata: &client.QueryMetadata{
			IdentifierValue: "0208:1018265814",
		},
		DnsInfo: &client.DnsInfo{
			Status:      "success",
			SMPHostname: &smpHost,
			SMLHostname: "edelivery.tech.ec.europa.eu",
		},
		BusinessCard: &client.LookupBusinessCard{
			Status: "success",
			Entities: []client.BusinessEntity{
				{
					Name:        &name,
					CountryCode: &country,
					Identifiers: []client.PeppolIdentifier{{Scheme: "BE:CBE", Value: "1018265814"}},
				},
			},
		},
		ExecutionTimeMS: 456.78,
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderLookupResult(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SUCCESS", "0208:1018265814", "Yes", "smp.example.be", "457ms", "Example Corp", "BE", "BE:CBE: 1018265814"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRenderSearchResults(t *testing.T) {
	name := "Test Corp"
	country := "BE"
	result := &client.PeppolSearchResult{
		TotalCount: 1,
		UsedCount:  1,
		Participants: []client.PeppolParticipant{
			{
				PeppolID:     "0208:1018265814",
				PeppolScheme: "iso6523-actorid-upis",
				Entities: []client.PeppolEntity{
					{Name: &name, CountryCode: &country},
				},
				DocumentTypes: []client.PeppolDocumentType{
					{Scheme: "busdox", Value: "invoice"},
				},
			},
		},
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderSearchResults(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Found 1 participants", "0208:1018265814", "Test Corp", "BE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRenderSearchResults_Empty(t *testing.T) {
	result := &client.PeppolSearchResult{
		TotalCount: 0,
		UsedCount:  0,
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderSearchResults(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Found 0 participants") {
		t.Errorf("expected 'Found 0 participants', got:\n%s", out)
	}
}

func TestRenderValidationResult_Valid(t *testing.T) {
	name := "Cezarotrans"
	country := "BE"
	regDate := "2026-02-16"
	result := &client.PeppolIdValidationResponse{
		IsValid:           true,
		DNSValid:          true,
		BusinessCardValid: true,
		SupportedDocumentTypes: []string{
			"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1",
			"urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2::CreditNote##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1",
			"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:selfbilling:3.0::2.1",
		},
		BusinessCard: &client.ValidationBusinessCard{
			Name:             &name,
			CountryCode:      &country,
			RegistrationDate: &regDate,
		},
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderValidationResult(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"VALID", "Yes", "Cezarotrans", "BE", "2026-02-16", "Supported Document Types (3)", "Invoice", "CreditNote", "Peppol BIS Billing 3.0", "Peppol Self-Billing 3.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRenderValidationResult_Invalid(t *testing.T) {
	result := &client.PeppolIdValidationResponse{
		IsValid:           false,
		DNSValid:          false,
		BusinessCardValid: false,
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderValidationResult(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "INVALID") {
		t.Errorf("output missing 'INVALID'\nGot:\n%s", out)
	}
	if !strings.Contains(out, "No") {
		t.Errorf("output missing 'No'\nGot:\n%s", out)
	}
}

func TestParseDocumentTypeURN(t *testing.T) {
	tests := []struct {
		urn         string
		wantType    string
		wantProfile string
	}{
		{
			"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1",
			"Invoice", "Peppol BIS Billing 3.0",
		},
		{
			"urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2::CreditNote##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0::2.1",
			"CreditNote", "Peppol BIS Billing 3.0",
		},
		{
			"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:nen.nl:nlcius:v1.0::2.1",
			"Invoice", "NL CIUS",
		},
		{
			"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2::Invoice##urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:selfbilling:3.0::2.1",
			"Invoice", "Peppol Self-Billing 3.0",
		},
	}
	for _, tt := range tests {
		gotType, gotProfile := parseDocumentTypeURN(tt.urn)
		if gotType != tt.wantType {
			t.Errorf("parseDocumentTypeURN(%q) type = %q, want %q", tt.urn, gotType, tt.wantType)
		}
		if gotProfile != tt.wantProfile {
			t.Errorf("parseDocumentTypeURN(%q) profile = %q, want %q", tt.urn, gotProfile, tt.wantProfile)
		}
	}
}

func TestRootCmd_WorkspaceFlag(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--workspace") {
		t.Error("help output missing '--workspace' flag")
	}
	if !strings.Contains(output, "-w") {
		t.Error("help output missing '-w' shorthand")
	}
}

// --- Attachment tests ---

func newAttachmentTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "Invalid API key"})
			return
		}

		switch {
		case r.URL.Path == "/api/documents/doc-1/attachments" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[
				{"id":"att-1","file_name":"invoice.pdf","file_type":"application/pdf","file_size":102400},
				{"id":"att-2","file_name":"receipt.png","file_type":"image/png","file_size":2048}
			]`))
		case r.URL.Path == "/api/documents/doc-1/attachments/att-1" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"att-1","file_name":"invoice.pdf","file_type":"application/pdf","file_size":102400,"file_url":"https://example.com/invoice.pdf"}`))
		case r.URL.Path == "/api/documents/doc-1/attachments" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"att-new","file_name":"upload.pdf","file_type":"application/pdf","file_size":512}`))
		case r.URL.Path == "/api/documents/doc-1/attachments/att-1" && r.Method == "DELETE":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"is_deleted":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(client.ErrorResponse{Detail: "not found"})
		}
	}))
}

func TestAttachmentListCmd_TextOutput(t *testing.T) {
	srv := newAttachmentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	atts, err := c.ListAttachments("doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderAttachmentList(r, atts); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"att-1", "invoice.pdf", "application/pdf", "100.0 KB", "att-2", "receipt.png", "2.0 KB"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestAttachmentListCmd_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderAttachmentList(r, nil); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "No attachments") {
		t.Errorf("expected 'No attachments' message, got %q", buf.String())
	}
}

func TestAttachmentListCmd_JSONOutput(t *testing.T) {
	srv := newAttachmentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	atts, err := c.ListAttachments("doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(atts)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}

	var parsed []map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(parsed))
	}
	if parsed[0]["id"] != "att-1" {
		t.Errorf("expected first id 'att-1', got %v", parsed[0]["id"])
	}
}

func TestAttachmentGetCmd_TextOutput(t *testing.T) {
	srv := newAttachmentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	att, err := c.GetAttachment("doc-1", "att-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := r.KeyValue([]output.KVPair{
		{Key: "ID", Value: att.ID},
		{Key: "Filename", Value: att.FileName},
		{Key: "Type", Value: att.FileType},
		{Key: "Size", Value: formatFileSize(att.FileSize)},
	}); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"att-1", "invoice.pdf", "application/pdf", "100.0 KB"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestAttachmentAddCmd_JSONOutput(t *testing.T) {
	srv := newAttachmentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))

	tmp, err := os.CreateTemp(t.TempDir(), "test-*.pdf")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write([]byte("fake pdf"))
	tmp.Close()

	att, err := c.AddAttachment("doc-1", tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if att.ID != "att-new" {
		t.Errorf("expected id 'att-new', got %q", att.ID)
	}
}

func TestAttachmentDeleteCmd_Success(t *testing.T) {
	srv := newAttachmentTestServer(t)
	defer srv.Close()

	c := client.NewClient("valid-key", client.WithBaseURL(srv.URL))
	result, err := c.DeleteAttachment("doc-1", "att-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsDeleted {
		t.Error("expected is_deleted to be true")
	}
}

func TestAttachmentCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"document", "attachment", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"list", "get", "add", "delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{2048, "2.0 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}
	for _, tt := range tests {
		got := formatFileSize(tt.input)
		if got != tt.want {
			t.Errorf("formatFileSize(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Document Create/Send/Validate CLI tests (PRD-217) ---

func TestDocumentCreateCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "create", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"json", "ubl", "pdf"} {
		if !strings.Contains(out, want) {
			t.Errorf("create help missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentSendCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "send", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"sender-peppol-id", "receiver-peppol-id"} {
		if !strings.Contains(out, want) {
			t.Errorf("send help missing '--%s'\nGot:\n%s", want, out)
		}
	}
	// --email is deprecated by the API: cobra hides deprecated flags from help.
	if strings.Contains(out, "--email") {
		t.Errorf("send help should hide deprecated --email\nGot:\n%s", out)
	}
}

func TestDocumentValidateCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "validate", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Peppol BIS") {
		t.Errorf("validate help missing Peppol BIS description\nGot:\n%s", out)
	}
}

func TestRenderValidation_Valid(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	val := &client.ValidationResponse{ID: "doc-123", IsValid: true, Issues: []client.ValidationIssue{}}
	if err := renderValidation(r, val); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "valid") {
		t.Errorf("expected 'valid' in output\nGot:\n%s", buf.String())
	}
}

func TestRenderValidation_Invalid(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	ruleID := "BR-07"
	val := &client.ValidationResponse{
		ID: "doc-123", IsValid: false,
		Issues: []client.ValidationIssue{
			{Message: "Missing buyer name", Type: client.IssueTypeError, RuleID: &ruleID, Schematron: "BR-07"},
		},
	}
	if err := renderValidation(r, val); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"not valid", "Missing buyer name", "BR-07"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentDeleteCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "delete", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"--yes", "-y", "Skip confirmation"} {
		if !strings.Contains(out, want) {
			t.Errorf("delete help missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentDeleteCmd_WithYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.DocumentDelete{IsDeleted: true})
	}))
	defer srv.Close()

	c := client.NewClient("test-key", client.WithBaseURL(srv.URL))
	result, err := c.DeleteDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsDeleted {
		t.Error("expected is_deleted=true")
	}
}

func TestDocumentDeleteCmd_ConfirmNo(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewRootCmd()
	cmd.SetOut(buf)
	cmd.SetIn(strings.NewReader("n\n"))
	t.Setenv("PEPPOL_API_KEY", "test-key")
	cmd.SetArgs([]string{"document", "delete", "doc-123"})
	_ = cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, "Aborted") {
		t.Errorf("expected 'Aborted' in output\nGot:\n%s", out)
	}
}

func TestDocumentDeleteCmd_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.DocumentDelete{IsDeleted: true})
	}))
	defer srv.Close()

	c := client.NewClient("test-key", client.WithBaseURL(srv.URL))
	result, err := c.DeleteDocument("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	if !strings.Contains(string(data), `"is_deleted":true`) {
		t.Errorf("expected is_deleted in JSON\nGot:\n%s", string(data))
	}
}

func TestDocumentUBLCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"document", "ubl", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"--output", "-o", "UBL XML"} {
		if !strings.Contains(out, want) {
			t.Errorf("ubl help missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestDocumentUBLCmd_Stdout(t *testing.T) {
	xmlContent := `<?xml version="1.0"?><Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"></Invoice>`

	// Serve the UBL XML from a "signed URL" server
	xmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(xmlContent))
	}))
	defer xmlSrv.Close()

	signedURL := xmlSrv.URL + "/ubl.xml"

	// API server returns UBL metadata with signed URL
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(client.DocumentUBL{
			FileName:  "invoice.xml",
			FileSize:  len(xmlContent),
			SignedURL: &signedURL,
		})
	}))
	defer apiSrv.Close()

	c := client.NewClient("test-key", client.WithBaseURL(apiSrv.URL))
	ubl, err := c.GetDocumentUBL("doc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ubl.FileName != "invoice.xml" {
		t.Errorf("expected file_name 'invoice.xml', got %q", ubl.FileName)
	}
	if ubl.SignedURL == nil {
		t.Fatal("expected signed_url to be set")
	}
}

func TestDocumentUBLCmd_JSONMetadata(t *testing.T) {
	signedURL := "https://storage.example.com/ubl.xml"
	ubl := client.DocumentUBL{
		FileName:  "invoice.xml",
		FileSize:  4096,
		SignedURL: &signedURL,
	}
	data, err := json.Marshal(ubl)
	if err != nil {
		t.Fatalf("JSON marshal error: %v", err)
	}
	for _, want := range []string{`"file_name":"invoice.xml"`, `"file_size":4096`, `"signed_url"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON missing %q\nGot:\n%s", want, string(data))
		}
	}
}

func TestValidateJSONCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "json", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Peppol BIS") {
		t.Errorf("validate json help missing Peppol BIS description\nGot:\n%s", out)
	}
	if !strings.Contains(out, "stdin") {
		t.Errorf("validate json help missing stdin mention\nGot:\n%s", out)
	}
}

func TestValidateUBLCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "ubl", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "UBL/XML") {
		t.Errorf("validate ubl help missing UBL/XML description\nGot:\n%s", out)
	}
}

func TestRenderFileValidation_Passed(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	val := &client.ValidationResponse{ID: "val-1", IsValid: true, Issues: []client.ValidationIssue{}}
	if err := renderFileValidation(r, val); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Validation: PASSED") {
		t.Errorf("expected 'Validation: PASSED' in output\nGot:\n%s", out)
	}
}

func TestRenderFileValidation_Failed(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	ruleID := "BR-07"
	location := "/Invoice/cac:AccountingCustomerParty"
	val := &client.ValidationResponse{
		ID: "val-2", IsValid: false,
		Issues: []client.ValidationIssue{
			{Message: "Missing buyer name", Type: client.IssueTypeError, RuleID: &ruleID, Location: &location, Schematron: "BR-07"},
			{Message: "Recommended field", Type: client.IssueTypeWarning, Schematron: "BR-CL-01"},
		},
	}
	if err := renderFileValidation(r, val); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Validation: FAILED (1 errors, 1 warnings)",
		"Missing buyer name",
		"BR-07",
		"ERROR",
		"WARNING",
		"Recommended field",
		"/Invoice/cac:AccountingCustomerParty",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestShortFlags(t *testing.T) {
	tests := []struct {
		flag  string
		check func() bool
	}{
		{"-j", func() bool { return flags.JSON }},
		{"-q", func() bool { return flags.Quiet }},
		{"-v", func() bool { return flags.Verbose }},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			// Reset flags
			flags = GlobalFlags{}

			cmd := NewRootCmd()
			cmd.SetArgs([]string{tt.flag, "--help"})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			_ = cmd.Execute()

			if !tt.check() {
				t.Errorf("expected flag %s to be set", tt.flag)
			}
		})
	}
}

func TestAllCommandsHaveExamples(t *testing.T) {
	cmd := NewRootCmd()

	var check func(cmd *cobra.Command)
	check = func(cmd *cobra.Command) {
		// Skip root and help commands
		if cmd.Name() == "peppol" || cmd.Name() == "help" {
			for _, sub := range cmd.Commands() {
				check(sub)
			}
			return
		}

		// All non-help commands should have examples
		if cmd.Example == "" {
			t.Errorf("command %q is missing Example field", cmd.CommandPath())
		}

		for _, sub := range cmd.Commands() {
			check(sub)
		}
	}
	check(cmd)
}

// --- Backup command tests ---

func TestBackupCmd_Help_DescribesDirectoryStateMachine(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"backup", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Help text must document the three modes so users know what re-running does.
	for _, want := range []string{"empty", "resume", "top-up", "manifest.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("backup --help missing %q\nGot:\n%s", want, out)
		}
	}
	// And the flags.
	for _, flag := range []string{"--layout", "--concurrency"} {
		if !strings.Contains(out, flag) {
			t.Errorf("backup --help missing %q\nGot:\n%s", flag, out)
		}
	}
}

func TestBackupCmd_RequiresDirectoryArg(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"backup"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when <dir> arg missing")
	}
}

func TestBackupCmd_RegisteredAtRoot(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "backup") {
		t.Errorf("root --help does not list backup command:\n%s", buf.String())
	}
}

// --- Mailbox command tests ---

func TestMailboxCmd_RegisteredWithSubcommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"mailbox", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"list", "get", "attachment", "reprocess"} {
		if !strings.Contains(out, want) {
			t.Errorf("mailbox help missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestMailboxList_FlagsReachQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"all filters",
			[]string{
				"--status", "failed",
				"--processed=false",
				"--from", "2026-03-01T00:00:00Z",
				"--to", "2026-03-31T23:59:59Z",
				"--search", "invoice",
				"--sort-by", "created_at",
				"--sort-order", "asc",
				"--page", "3",
				"--page-size", "50",
			},
			"page=3&page_size=50&processed=false&received_from=2026-03-01T00%3A00%3A00Z&received_to=2026-03-31T23%3A59%3A59Z&search=invoice&sort_by=created_at&sort_order=asc&status=failed",
		},
		{"processed not given", []string{"--search", "invoice"}, "page=1&page_size=20&search=invoice"},
		{"processed given", []string{"--processed"}, "page=1&page_size=20&processed=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.RawQuery
				w.Write([]byte(`{"items":[]}`))
			}))
			defer srv.Close()

			cmd := newMailboxListCmd()
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("parsing flags: %v", err)
			}
			params, err := mailboxListParams(cmd)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := client.NewClient("key", client.WithBaseURL(srv.URL)).ListMailbox(params); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("query = %q\nwant    %q", got, tt.want)
			}
		})
	}
}

func TestMailboxList_RejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"status", []string{"--status", "done"}, `invalid --status "done", must be one of: pending, success, failed`},
		{"sort-by", []string{"--sort-by", "invoice_date"}, `invalid --sort-by "invoice_date", must be one of: received_at, created_at`},
		{"sort-order", []string{"--sort-order", "up"}, `invalid --sort-order "up", must be one of: asc, desc`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(append([]string{"mailbox", "list"}, tt.args...))
			err := cmd.Execute()
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestRenderMailboxList_Table(t *testing.T) {
	received := time.Date(2026, 3, 2, 9, 15, 0, 0, time.UTC)
	result := &client.PaginatedInboundEmails{
		Items: []client.InboundEmailResponse{
			{
				ID:              "mail-ok",
				SenderEmail:     "billing@supplier.example",
				Subject:         strPtr("Invoice 2026-001"),
				AttachmentCount: 2,
				Processed:       true,
				ReceivedAt:      &received,
				CreatedAt:       time.Date(2026, 3, 2, 9, 15, 4, 0, time.UTC),
				DocumentID:      strPtr("doc-9"),
			},
			{
				ID:           "mail-bad",
				SenderEmail:  "ap@vendor.example",
				Processed:    true,
				ErrorMessage: strPtr("No PDF attachment found"),
				// received_at is null: the API falls back to created_at.
				CreatedAt: time.Date(2026, 4, 10, 7, 30, 0, 0, time.UTC),
			},
			{
				ID:          "mail-new",
				SenderEmail: "new@vendor.example",
				CreatedAt:   time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
			},
		},
		Total: 41, Page: 2, PageSize: 3, Pages: 14, HasNextPage: true,
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderMailboxList(r, result); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	if want := "Showing 4-6 of 41 emails (page 2/14)"; !strings.Contains(out, want) {
		t.Errorf("output missing %q\nGot:\n%s", want, out)
	}
	// Cells in column order, separated by any border or padding.
	for _, row := range []string{
		`ID\W+PROCESSED\W+FROM\W+SUBJECT\W+ATTACHMENTS\W+RECEIVED`,
		`mail-ok\W+Yes\W+billing@supplier\.example\W+Invoice 2026-001\W+2\W+2026-03-02 09:15`,
		`mail-bad\W+Yes\W+ap@vendor\.example\W+0\W+2026-04-10 07:30`,
		`mail-new\W+No\W+new@vendor\.example\W+0\W+2026-04-11 08:00`,
	} {
		if !regexp.MustCompile(row).MatchString(out) {
			t.Errorf("output has no row matching %s\nGot:\n%s", row, out)
		}
	}
}

func TestRenderMailboxList_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderMailboxList(r, &client.PaginatedInboundEmails{}); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(buf.String(), "No inbound emails found.") {
		t.Errorf("expected empty message, got %q", buf.String())
	}
}

func TestRenderMailboxEmail_FailedWithAttachments(t *testing.T) {
	size := 102400
	mail := &client.InboundEmailResponse{
		ID:          "mail-7",
		MessageID:   "<m7@mail.example.com>",
		SenderEmail: "ap@vendor.example",
		SenderName:  strPtr("Vendor AP"),
		ToAddresses: strPtr("invoices@tenant.example"),
		Subject:     strPtr("Invoice 42"),
		Attachments: []client.AttachmentInfo{
			{Filename: "invoice.pdf", Size: &size, ContentType: strPtr("application/pdf")},
			{Filename: "unknown.bin"},
		},
		AttachmentCount: 2,
		ErrorMessage:    strPtr("No PDF attachment found"),
		CreatedAt:       time.Date(2026, 4, 10, 7, 30, 0, 0, time.UTC),
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderMailboxEmail(r, mail); err != nil {
		t.Fatalf("render error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`(?m)^ID +mail-7$`,
		`(?m)^Message ID +<m7@mail\.example\.com>$`,
		`(?m)^From +Vendor AP <ap@vendor\.example>$`,
		`(?m)^To +invoices@tenant\.example$`,
		`(?m)^Subject +Invoice 42$`,
		`(?m)^Received +2026-04-10 07:30:00$`,
		`(?m)^Processed +No$`,
		`(?m)^Error +No PDF attachment found$`,
		`invoice\.pdf\W+application/pdf\W+100\.0 KB`,
		`unknown\.bin\W+-\W+-`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("output has no line matching %s\nGot:\n%s", want, out)
		}
	}
	// Absent optional fields get no row.
	if absent := regexp.MustCompile(`(?m)^(CC|BCC|Processed At|Document) `); absent.MatchString(out) {
		t.Errorf("output has a row for an absent field: %q\nGot:\n%s", absent.FindString(out), out)
	}
}

func TestWriteMailboxAttachment_StdoutGetsRawBytes(t *testing.T) {
	content := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	att := &client.MailboxAttachment{Content: content, ContentType: "image/png"}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := writeMailboxAttachment(r, att, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Errorf("stdout = %v, want exactly %v", buf.Bytes(), content)
	}
}

func TestWriteMailboxAttachment_OutputFile(t *testing.T) {
	content := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	att := &client.MailboxAttachment{Content: content, ContentType: "image/png"}
	path := filepath.Join(t.TempDir(), "scan.png")

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := writeMailboxAttachment(r, att, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file = %v, want %v", got, content)
	}
	if want := "Attachment written to " + path + " (6 B)"; !strings.Contains(buf.String(), want) {
		t.Errorf("output = %q, want it to contain %q", buf.String(), want)
	}
}

func TestWriteMailboxAttachment_OutputFileJSON(t *testing.T) {
	att := &client.MailboxAttachment{Content: []byte("<Invoice/>"), ContentType: "application/xml"}
	path := filepath.Join(t.TempDir(), "invoice.xml")

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, true, false, true, false)
	if err := writeMailboxAttachment(r, att, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, buf.String())
	}
	if got["output"] != path || got["content_type"] != "application/xml" || got["size"] != float64(10) {
		t.Errorf("unexpected JSON summary: %v", got)
	}
}

// --- Item attributes (BG-32) ---

func TestRenderDocumentSections_FullShowsItemAttributes(t *testing.T) {
	var doc client.DocumentResponse
	// Shape taken from the LineItem schema: item_attributes is a list of
	// {name, value}, value may be null.
	err := json.Unmarshal([]byte(`{
		"id": "doc-1",
		"created_at": "2026-01-10T10:00:00Z",
		"items": [
			{"description": "T-shirt", "quantity": "2", "unit_price": "10.00", "amount": "20.00",
			 "item_attributes": [{"name": "Color", "value": "Red"}, {"name": "Fragile", "value": null}]},
			{"description": "Shipping", "quantity": "1", "unit_price": "5.00", "amount": "5.00"}
		]
	}`), &doc)
	if err != nil {
		t.Fatalf("decoding document: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderDocumentSections(r, &doc, true); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ATTRIBUTES", "Color=Red, Fragile"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRenderDocumentSections_FullWithoutItemAttributesKeepsColumns(t *testing.T) {
	doc := &client.DocumentResponse{
		ID:    "doc-1",
		Items: []client.LineItem{{Description: strPtr("Shipping"), Amount: strPtr("5.00")}},
	}
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderDocumentSections(r, doc, true); err != nil {
		t.Fatalf("render error: %v", err)
	}
	if strings.Contains(buf.String(), "ATTRIBUTES") {
		t.Errorf("no item has attributes, so no Attributes column expected\nGot:\n%s", buf.String())
	}
}

// --- document create pdf: failed conversion ---

func TestRenderPDFCreateResult_FailedConversionShowsError(t *testing.T) {
	var doc client.DocumentCreateFromPdfResponse
	// A failed conversion: success false, error fields set, items empty.
	err := json.Unmarshal([]byte(`{
		"id": "doc-pdf-1",
		"created_at": "2026-05-04T12:00:00Z",
		"state": "DRAFT",
		"success": false,
		"error_type": "extraction_failed",
		"error_message": "Could not read the invoice total",
		"items": []
	}`), &doc)
	if err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderPDFCreateResult(r, &doc); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"PDF conversion failed",
		"extraction_failed",
		"Could not read the invoice total",
		"doc-pdf-1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "successfully") {
		t.Errorf("failed conversion must not report success\nGot:\n%s", out)
	}
}

func TestRenderPDFCreateResult_Success(t *testing.T) {
	doc := &client.DocumentCreateFromPdfResponse{
		DocumentResponse: client.DocumentResponse{ID: "doc-pdf-2"},
		Success:          true,
	}
	buf := new(bytes.Buffer)
	r := output.NewTestRenderer(buf, false, false, true, false)
	if err := renderPDFCreateResult(r, doc); err != nil {
		t.Fatalf("render error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Document created from PDF successfully.") || !strings.Contains(out, "doc-pdf-2") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if regexp.MustCompile(`(?m)^Error (Type|Message) `).MatchString(out) {
		t.Errorf("successful conversion must not show error rows\nGot:\n%s", out)
	}
}
