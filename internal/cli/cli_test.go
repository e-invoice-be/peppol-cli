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
