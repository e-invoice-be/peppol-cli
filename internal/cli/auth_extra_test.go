package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/e-invoice-be/peppol-cli/internal/config"
)

// --- resolveKey ---

func TestResolveKey_NoCredentialsIsAuthError(t *testing.T) {
	isolateConfig(t)
	key, err := resolveKey()
	if key != "" {
		t.Errorf("key = %q, want empty", key)
	}
	if got := exitCode(err); got != 2 {
		t.Fatalf("exit code = %d, want 2 (err: %v)", got, err)
	}
	if !strings.Contains(err.Error(), "peppol auth") {
		t.Errorf("error should tell the user to run 'peppol auth', got %q", err)
	}
}

func TestResolveKey_EnvVarBeatsStoredWorkspaceKey(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "alpha", map[string]string{"alpha": "stored-key"})
	t.Setenv("PEPPOL_API_KEY", "env-key")

	key, err := resolveKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" {
		t.Errorf("key = %q, want env-key", key)
	}
}

func TestResolveKey_ActiveWorkspaceKey(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "beta", map[string]string{"alpha": "alpha-key", "beta": "beta-key"})

	key, err := resolveKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "beta-key" {
		t.Errorf("key = %q, want beta-key", key)
	}
}

func TestResolveKey_DefaultCredentialsWithoutWorkspace(t *testing.T) {
	dir := isolateConfig(t)
	if err := config.NewFileKeyring(dir).Set("legacy-key"); err != nil {
		t.Fatal(err)
	}
	key, err := resolveKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "legacy-key" {
		t.Errorf("key = %q, want legacy-key", key)
	}
}

func TestResolveKey_CorruptConfigIsGeneralError(t *testing.T) {
	dir := isolateConfig(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("active_workspace: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveKey()
	if got := exitCode(err); got != 1 {
		t.Fatalf("exit code = %d, want 1 (err: %v)", got, err)
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("error = %q, want mention of loading config", err)
	}
}

func TestResolveKey_UnreadableCredentialsIsGeneralError(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "alpha", map[string]string{"alpha": ""})
	// A directory where the credentials file should be makes reading fail.
	if err := os.MkdirAll(filepath.Join(dir, "credentials-alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := resolveKey()
	if got := exitCode(err); got != 1 {
		t.Fatalf("exit code = %d, want 1 (err: %v)", got, err)
	}
	if !strings.Contains(err.Error(), "reading credentials") {
		t.Errorf("error = %q, want mention of reading credentials", err)
	}
}

func TestWorkspaceFlag_SelectsThatWorkspacesKeyEndToEnd(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Beta Tenant", "plan": "pro"})
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "alpha", map[string]string{"alpha": "bad-key", "beta": goodKey})

	// The active workspace's key is rejected by the API: auth exit code.
	wantCode(t, runCLI(t, "", "me"), 2)

	res := runCLI(t, "", "me", "-w", "beta")
	wantOK(t, res)
	wantContains(t, res.Out, "Beta Tenant", "pro")
	if got := api.last("/api/me/").Auth; got != "Bearer "+goodKey {
		t.Errorf("Authorization = %q, want beta's key", got)
	}
}

// --- me ---

func TestMe_TextOutputListsOptionalFields(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{
		"name": "Acme", "plan": "starter", "company_name": "Acme NV",
		"company_number": "0123456789", "company_tax_id": "BE0123456789",
		"peppol_ids": []string{"0208:0123456789", "9925:BE0123456789"},
	})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "me")
	wantOK(t, res)
	wantContains(t, res.Out, "Acme NV", "0123456789", "BE0123456789", "starter", "0208:0123456789", "9925:BE0123456789")
}

func TestMe_JSONOutput(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Acme", "plan": "pro"})
	useEnvKey(t, goodKey)

	res := runCLI(t, "", "me", "--json")
	wantOK(t, res)
	m := decodeJSON(t, res.Out)
	if m["name"] != "Acme" || m["plan"] != "pro" {
		t.Errorf("unexpected JSON: %v", m)
	}
}

func TestMe_InvalidKeyIsExitCode2(t *testing.T) {
	newFakeAPI(t)
	useEnvKey(t, "wrong-key")
	res := runCLI(t, "", "me")
	wantCode(t, res, 2)
	wantContains(t, res.Err.Error(), "authentication failed")
}

func TestMe_ServerErrorIsExitCode1WithDetail(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusInternalServerError, map[string]string{"detail": "database down"})
	useEnvKey(t, goodKey)
	res := runCLI(t, "", "me")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "500", "database down")
}

func TestMe_NotAuthenticatedNeverCallsAPI(t *testing.T) {
	api := newFakeAPI(t)
	isolateConfig(t)
	wantCode(t, runCLI(t, "", "me"), 2)
	if n := len(api.requests()); n != 0 {
		t.Errorf("expected no API requests without a key, got %d", n)
	}
}

func TestMe_VerboseLogsRequestsToStderr(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Acme", "plan": "pro"})
	useEnvKey(t, goodKey)

	stderr := captureStderr(t, func() { wantOK(t, runCLI(t, "", "me", "-v")) })
	wantContains(t, stderr, ">> GET", "/api/me/", "<< 200")
	if strings.Contains(stderr, goodKey) {
		t.Error("verbose output must not leak the API key")
	}
}

// captureStderr runs fn with os.Stderr redirected to a temp file and returns
// what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = orig }()
	fn()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// --- auth ---

// noBrowser keeps runAuth from launching a real browser: pkg/browser execs
// xdg-open/open, which cannot be found on an empty PATH.
func noBrowser(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func TestAuth_StoresKeyUnderSlugifiedTenantWorkspace(t *testing.T) {
	noBrowser(t)
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "My Company B.V.", "plan": "pro"})
	dir := isolateConfig(t)
	withStdin(t, "  "+goodKey+"  \n")

	res := runCLI(t, "", "auth")
	wantOK(t, res)
	wantContains(t, res.Out, "api-settings", "Validating... OK", "Authenticated as My Company B.V. (workspace: my-company-b-v)")

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "my-company-b-v" {
		t.Errorf("active workspace = %q", cfg.ActiveWorkspace)
	}
	if cfg.Workspaces["my-company-b-v"].Name != "My Company B.V." {
		t.Errorf("workspace tenant = %q", cfg.Workspaces["my-company-b-v"].Name)
	}
	key, err := config.NewFileKeyringForWorkspace(dir, "my-company-b-v").Get()
	if err != nil {
		t.Fatal(err)
	}
	if key != goodKey {
		t.Errorf("stored key = %q, want trimmed %q", key, goodKey)
	}
	info, err := os.Stat(filepath.Join(dir, "credentials-my-company-b-v"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file mode = %o, want 600", perm)
	}
}

func TestAuth_WorkspaceFlagAndQuiet(t *testing.T) {
	noBrowser(t)
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Acme", "plan": "pro"})
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "old", map[string]string{"old": "old-key"})
	withStdin(t, goodKey+"\n")

	res := runCLI(t, "", "auth", "-w", "prod", "-q")
	wantOK(t, res)
	if strings.Contains(res.Out, "Validating") {
		t.Errorf("--quiet should suppress progress, got %q", res.Out)
	}
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "prod" {
		t.Errorf("active = %q, want prod", cfg.ActiveWorkspace)
	}
	if _, ok := cfg.Workspaces["old"]; !ok {
		t.Error("existing workspace should be kept")
	}
}

func TestAuth_TenantNameWithoutSlugCharsUsesDefault(t *testing.T) {
	noBrowser(t)
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "***", "plan": "pro"})
	dir := isolateConfig(t)
	withStdin(t, goodKey+"\n")

	wantOK(t, runCLI(t, "", "auth"))
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "default" {
		t.Errorf("active = %q, want default", cfg.ActiveWorkspace)
	}
}

func TestAuth_EmptyKeyIsExitCode1(t *testing.T) {
	noBrowser(t)
	isolateConfig(t)
	withStdin(t, "   \n")
	res := runCLI(t, "", "auth")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "no API key provided")
}

func TestAuth_NoInputIsExitCode1(t *testing.T) {
	noBrowser(t)
	isolateConfig(t)
	withStdin(t, "")
	res := runCLI(t, "", "auth")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "reading input")
}

func TestAuth_RejectedKeyIsExitCode2AndStoresNothing(t *testing.T) {
	noBrowser(t)
	newFakeAPI(t)
	dir := isolateConfig(t)
	withStdin(t, "bad-key\n")

	res := runCLI(t, "", "auth")
	wantCode(t, res, 2)
	wantContains(t, res.Err.Error(), "validation failed")
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Errorf("config should not be written after a rejected key (stat err: %v)", err)
	}
}

// BUG: runAuth treats an unreadable config as empty and saves over it, so
// every other workspace is silently dropped.
func TestAuth_CorruptConfigIsNotOverwritten(t *testing.T) {
	noBrowser(t)
	skipKnownBug(t, "auth.go runAuth replaces an unparsable config.yaml with a fresh one, losing all workspaces")
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Acme", "plan": "pro"})
	dir := isolateConfig(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	corrupt := "active_workspace: prod\nworkspaces:\n  prod: {name: Prod\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	withStdin(t, goodKey+"\n")
	res := runCLI(t, "", "auth")
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Err == nil && string(data) != corrupt {
		t.Errorf("config was overwritten without an error:\n%s", data)
	}
}

// --- auth status / logout ---

func TestAuthStatus_ShowsWorkspaceAndMaskedKey(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "prod", map[string]string{"prod": "sk_live_abcdef1234"})

	res := runCLI(t, "", "auth", "status")
	wantOK(t, res)
	wantContains(t, res.Out, "Tenant prod", "workspace: prod", "****...1234")
	if strings.Contains(res.Out, "sk_live_abcdef1234") {
		t.Error("status must not print the full key")
	}
}

func TestAuthStatus_EnvKeyWithoutWorkspace(t *testing.T) {
	useEnvKey(t, "env-key-9876")
	res := runCLI(t, "", "auth", "status")
	wantOK(t, res)
	wantContains(t, res.Out, "Authenticated (key: ****...9876)")
}

func TestAuthStatus_UnreadableCredentialsIsExitCode1(t *testing.T) {
	dir := isolateConfig(t)
	if err := os.MkdirAll(filepath.Join(dir, "credentials"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantCode(t, runCLI(t, "", "auth", "status"), 1)
}

func TestAuthLogout_RemovesActiveWorkspaceAndKey(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "prod", map[string]string{"prod": "k1", "test": "k2"})

	res := runCLI(t, "", "auth", "logout")
	wantOK(t, res)
	wantContains(t, res.Out, "Logged out")

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "" {
		t.Errorf("active = %q, want empty", cfg.ActiveWorkspace)
	}
	if _, ok := cfg.Workspaces["prod"]; ok {
		t.Error("prod should be removed")
	}
	if _, ok := cfg.Workspaces["test"]; !ok {
		t.Error("test should be kept")
	}
	if key, _ := config.NewFileKeyringForWorkspace(dir, "prod").Get(); key != "" {
		t.Errorf("prod key should be removed, got %q", key)
	}
	if key, _ := config.NewFileKeyringForWorkspace(dir, "test").Get(); key != "k2" {
		t.Errorf("test key should be kept, got %q", key)
	}
}

func TestAuthLogout_WorkspaceFlagKeepsActive(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "prod", map[string]string{"prod": "k1", "test": "k2"})

	wantOK(t, runCLI(t, "", "auth", "logout", "-w", "test"))
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "prod" {
		t.Errorf("active = %q, want prod", cfg.ActiveWorkspace)
	}
	if _, ok := cfg.Workspaces["test"]; ok {
		t.Error("test should be removed")
	}
}

// --- workspace add ---

func TestWorkspaceAdd_ValidatesAndStoresKey(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Staging Tenant", "plan": "pro"})
	dir := isolateConfig(t)
	withStdin(t, goodKey+"\n")

	res := runCLI(t, "", "workspace", "add", "staging")
	wantOK(t, res)
	wantContains(t, res.Out, `Workspace "staging" added (tenant: Staging Tenant)`)

	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "staging" {
		t.Errorf("first workspace should become active, got %q", cfg.ActiveWorkspace)
	}
	if key, _ := config.NewFileKeyringForWorkspace(dir, "staging").Get(); key != goodKey {
		t.Errorf("stored key = %q", key)
	}
}

func TestWorkspaceAdd_ExistingNameIsExitCode1(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "prod", map[string]string{"prod": "k"})
	res := runCLI(t, "", "ws", "add", "prod")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "already exists")
}

func TestWorkspaceAdd_EmptyKeyIsExitCode1(t *testing.T) {
	isolateConfig(t)
	withStdin(t, "\n")
	wantCode(t, runCLI(t, "", "workspace", "add", "x"), 1)
}

func TestWorkspaceAdd_RejectedKeyIsExitCode2(t *testing.T) {
	newFakeAPI(t)
	dir := isolateConfig(t)
	withStdin(t, "bad\n")
	wantCode(t, runCLI(t, "", "workspace", "add", "x"), 2)
	if key, _ := config.NewFileKeyringForWorkspace(dir, "x").Get(); key != "" {
		t.Errorf("rejected key must not be stored, got %q", key)
	}
}

func TestWorkspaceAdd_RequiresName(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "workspace", "add")
	if res.Err == nil {
		t.Fatal("expected an argument error")
	}
}

// BUG: workspace names are used unchecked in credential file names, so a
// name with path separators writes the API key outside the config dir.
func TestWorkspaceAdd_RejectsPathTraversalName(t *testing.T) {
	skipKnownBug(t, "workspace.go runWorkspaceAdd / auth -w accept names like 'x/../../escaped'; keyring.go joins them into the credentials path")
	api := newFakeAPI(t)
	api.json("GET /api/me/", http.StatusOK, map[string]any{"name": "Acme", "plan": "pro"})
	dir := isolateConfig(t)
	withStdin(t, goodKey+"\n")
	// The key lands in <config>/credentials-x/../../escaped = <config>/../escaped.
	res := runCLI(t, "", "workspace", "add", "x/../../escaped")
	if res.Err == nil {
		t.Errorf("expected invalid workspace name error")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped")); err == nil {
		t.Error("API key written outside the config directory")
	}
}

// --- workspace list / use / remove error paths ---

func TestWorkspaceCommands_CorruptConfigIsExitCode1(t *testing.T) {
	dir := isolateConfig(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("{{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"workspace", "list"},
		{"workspace", "use", "a"},
		{"workspace", "remove", "a"},
		{"workspace", "add", "a"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			res := runCLI(t, "", args...)
			wantCode(t, res, 1)
			wantContains(t, res.Err.Error(), "loading config")
		})
	}
}

func TestWorkspaceRemove_UnknownIsExitCode1(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "workspace", "remove", "ghost")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), "not found")
}

func TestWorkspaceRemove_LastActiveWorkspace(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "only", map[string]string{"only": "k"})
	wantOK(t, runCLI(t, "", "workspace", "remove", "only"))
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "" || len(cfg.Workspaces) != 0 {
		t.Errorf("config not emptied: %+v", cfg)
	}
}

func TestWorkspaceList_TableMarksActive(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "b", map[string]string{"a": "", "b": ""})
	res := runCLI(t, "", "workspace", "list")
	wantOK(t, res)
	var activeLine string
	for _, line := range strings.Split(res.Out, "\n") {
		if strings.Contains(line, "*") {
			activeLine = line
		}
	}
	if !strings.Contains(activeLine, "Tenant b") {
		t.Errorf("active marker should be on workspace b, output:\n%s", res.Out)
	}
}

func TestWorkspaceList_EmptyJSONIsArray(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "workspace", "list", "--json")
	wantOK(t, res)
	if strings.TrimSpace(res.Out) != "[]" {
		t.Errorf("want [], got %q", res.Out)
	}
}
