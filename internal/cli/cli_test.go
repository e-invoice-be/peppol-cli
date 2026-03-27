package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
