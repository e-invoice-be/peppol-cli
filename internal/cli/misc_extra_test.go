package cli

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- backup ---

func TestBackup_InvalidLayoutFailsBeforeAuth(t *testing.T) {
	isolateConfig(t)
	res := runCLI(t, "", "backup", t.TempDir(), "--layout", "zip")
	wantCode(t, res, 1)
	wantContains(t, res.Err.Error(), `invalid --layout "zip"`)
}

func TestBackup_NoCredentialsIsCode2(t *testing.T) {
	isolateConfig(t)
	wantCode(t, runCLI(t, "", "backup", t.TempDir()), 2)
}

func TestBackup_EmptyAccountWritesManifest(t *testing.T) {
	api := newFakeAPI(t)
	for _, p := range []string{"/api/inbox/", "/api/outbox/", "/api/drafts/"} {
		api.json("GET "+p, http.StatusOK, docPage())
	}
	useEnvKey(t, goodKey)

	dir := filepath.Join(t.TempDir(), "bk")
	var res cliResult
	captureStderr(t, func() { res = runCLI(t, "", "backup", dir, "--layout", "tree", "-q") })
	wantOK(t, res)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Errorf("manifest.json not written: %v", err)
	}

	// Second run is a top-up of a complete backup.
	captureStderr(t, func() { res = runCLI(t, "", "backup", dir, "--layout", "tree", "--json", "-q") })
	wantOK(t, res)
	if res.Out != "" {
		t.Errorf("-q should suppress all stdout, got %q", res.Out)
	}
}

// With --json (and without -q) the "fetched <id>" and "done: ..." progress
// lines are suppressed so stdout is exactly the JSON summary.
func TestBackup_JSONOutputIsParseable(t *testing.T) {
	api := newFakeAPI(t)
	for _, p := range []string{"/api/inbox/", "/api/outbox/", "/api/drafts/"} {
		api.json("GET "+p, http.StatusOK, docPage())
	}
	useEnvKey(t, goodKey)
	var res cliResult
	captureStderr(t, func() { res = runCLI(t, "", "backup", t.TempDir(), "--json") })
	wantOK(t, res)
	if decodeJSON(t, res.Out)["documents_fetched"] != float64(0) {
		t.Errorf("unexpected JSON: %s", res.Out)
	}
}

// In text mode the per-document progress lines and the "done:" line still
// appear on stdout, ahead of the summary.
func TestBackup_TextModeShowsProgressOnStdout(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/inbox/", http.StatusOK, docPage(sampleDoc("in-1")))
	api.json("GET /api/outbox/", http.StatusOK, docPage())
	api.json("GET /api/drafts/", http.StatusOK, docPage())
	api.json("GET /api/documents/in-1", http.StatusOK, sampleDoc("in-1"))
	api.json("GET /api/documents/in-1/timeline", http.StatusOK, map[string]any{"document_id": "in-1", "events": []any{}})
	api.json("GET /api/documents/in-1/attachments", http.StatusOK, []any{})
	api.json("GET /api/documents/in-1/ubl", http.StatusOK, map[string]any{
		"id": "in-1", "file_name": "in-1.xml", "file_size": 3, "signed_url": api.fileURL("in-1.xml"),
	})
	api.handle("GET /files/in-1.xml", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<x/>")) })
	useEnvKey(t, goodKey)

	var stdout, stderr string
	var err error
	captureStderr(t, func() { stdout, stderr, err = runCLISplit(t, "", "backup", t.TempDir()) })
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	wantContains(t, stdout, "fetched in-1\n", "done: 1 fetched, 0 refreshed\n", "New documents")
	if strings.Contains(stderr, "fetched") {
		t.Errorf("progress leaked to stderr: %q", stderr)
	}
}

func TestBackup_ArchivesDocumentsAndReportsCounts(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /api/inbox/", http.StatusOK, docPage(sampleDoc("in-1")))
	api.json("GET /api/outbox/", http.StatusOK, docPage())
	api.json("GET /api/drafts/", http.StatusOK, docPage())
	api.json("GET /api/documents/in-1", http.StatusOK, sampleDoc("in-1"))
	api.json("GET /api/documents/in-1/timeline", http.StatusOK, map[string]any{"document_id": "in-1", "events": []any{}})
	api.json("GET /api/documents/in-1/attachments", http.StatusOK, []any{})
	api.json("GET /api/documents/in-1/ubl", http.StatusOK, map[string]any{
		"id": "in-1", "file_name": "in-1.xml", "file_size": 3, "signed_url": api.fileURL("in-1.xml"),
	})
	api.handle("GET /files/in-1.xml", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<x/>")) })
	useEnvKey(t, goodKey)

	dir := t.TempDir()
	var res cliResult
	captureStderr(t, func() { res = runCLI(t, "", "backup", dir, "--concurrency", "2") })
	wantOK(t, res)
	wantContains(t, res.Out, dir, "flat", "New documents")

	found := false
	if err := filepath.Walk(dir, func(p string, _ os.FileInfo, err error) error {
		if err == nil && strings.Contains(filepath.Base(p), "in-1") {
			found = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Errorf("no files for document in-1 under %s", dir)
	}
}

func TestBackup_APIErrorIsCode1(t *testing.T) {
	api := newFakeAPI(t)
	for _, p := range []string{"/api/inbox/", "/api/outbox/", "/api/drafts/"} {
		api.json("GET "+p, http.StatusInternalServerError, map[string]string{"detail": "down"})
	}
	useEnvKey(t, goodKey)
	var res cliResult
	captureStderr(t, func() { res = runCLI(t, "", "backup", t.TempDir()) })
	wantCode(t, res, 1)
}

// --- execute (process exit code and error rendering) ---

// runExecute runs execute() with args as os.Args and returns the exit code
// and what was written to stderr.
func runExecute(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	origArgs := os.Args
	os.Args = append([]string{"peppol"}, args...)
	t.Cleanup(func() { os.Args = origArgs })

	origStdout := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	stderr = captureStderr(t, func() { code = execute() })
	return code, stderr
}

func TestExecute_ExitCodesAndErrorOutput(t *testing.T) {
	isolateConfig(t)

	code, stderr := runExecute(t, "me")
	if code != 2 {
		t.Errorf("no credentials: code = %d, want 2", code)
	}
	wantContains(t, stderr, "Error:")

	code, stderr = runExecute(t, "me", "--json")
	if code != 2 {
		t.Errorf("json: code = %d, want 2", code)
	}
	var payload struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
		t.Fatalf("stderr is not a JSON error: %v\n%s", err, stderr)
	}
	if payload.Code != 2 || payload.Error == "" {
		t.Errorf("payload = %+v", payload)
	}

	code, stderr = runExecute(t, "me", "-q")
	if code != 2 || stderr != "" {
		t.Errorf("quiet: code = %d, stderr = %q", code, stderr)
	}

	// Non-ExitError failures (unknown command, bad flag) map to code 1.
	code, _ = runExecute(t, "no-such-command")
	if code != 1 {
		t.Errorf("unknown command: code = %d, want 1", code)
	}

	code, _ = runExecute(t, "version")
	if code != 0 {
		t.Errorf("version: code = %d, want 0", code)
	}
}

func TestExecute_UnknownWorkspaceIsCode2(t *testing.T) {
	dir := isolateConfig(t)
	saveWorkspaces(t, dir, "a", map[string]string{"a": "key-a"})
	code, stderr := runExecute(t, "me", "-w", "nope")
	if code != 2 {
		t.Errorf("code = %d, want 2 (stderr %q)", code, stderr)
	}
}
