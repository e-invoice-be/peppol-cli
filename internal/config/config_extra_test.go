package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noHome makes ConfigDir fail: XDG_CONFIG_HOME and HOME are both empty.
func noHome(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
}

// skipIfRoot skips permission-based tests, which root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
}

func TestConfigDir_XDGTakesPrecedence(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(xdg, "peppol-cli"); got != want {
		t.Errorf("ConfigDir = %q, want %q", got, want)
	}
}

func TestConfigDir_FallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "peppol-cli"); got != want {
		t.Errorf("ConfigDir = %q, want %q", got, want)
	}
}

func TestConfigDir_NoHomeIsError(t *testing.T) {
	noHome(t)
	if _, err := ConfigDir(); err == nil || !strings.Contains(err.Error(), "home directory") {
		t.Errorf("err = %v, want a home directory error", err)
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFrom_ErrorPaths(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "workspaces: {a: [1, 2]}")
	if _, err := LoadFrom(dir); err == nil || !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("type mismatch: err = %v", err)
	}

	dir = t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "config.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(dir); err == nil || !strings.Contains(err.Error(), "reading config") {
		t.Errorf("directory: err = %v", err)
	}

	dir = t.TempDir()
	writeConfig(t, dir, "active_workspace: x\n")
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workspaces == nil {
		t.Error("Workspaces should be initialized")
	}
}

func TestLoadFrom_UnreadableFile(t *testing.T) {
	skipIfRoot(t)
	dir := t.TempDir()
	writeConfig(t, dir, "active_workspace: x\n")
	if err := os.Chmod(filepath.Join(dir, "config.yaml"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(dir); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("err = %v, want a permission error", err)
	}
}

func TestSaveTo_ErrorPaths(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveTo(filepath.Join(file, "sub"), &Config{}); err == nil || !strings.Contains(err.Error(), "creating config directory") {
		t.Errorf("err = %v", err)
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "config.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := SaveTo(dir, &Config{}); err == nil {
		t.Error("expected an error writing over a directory")
	}
}

func TestSaveTo_OmitsEmptyWorkspaces(t *testing.T) {
	dir := t.TempDir()
	if err := SaveTo(dir, &Config{ActiveWorkspace: ""}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "workspaces") {
		t.Errorf("empty workspaces should be omitted:\n%s", data)
	}
}

func TestRemoveWorkspace_LastActiveClearsActive(t *testing.T) {
	cfg := &Config{Workspaces: map[string]Workspace{}}
	if err := cfg.AddWorkspace("only", Workspace{}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.RemoveWorkspace("only"); err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveWorkspace != "" || len(cfg.Workspaces) != 0 {
		t.Errorf("cfg = %+v", cfg)
	}
}

// --- FileKeyring error paths ---

func TestFileKeyring_GetErrorWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "credentials"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileKeyring(dir).Get(); err == nil || !strings.Contains(err.Error(), "reading credentials file") {
		t.Errorf("err = %v", err)
	}
}

func TestFileKeyring_SetErrorWhenDirCannotBeCreated(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewFileKeyring(filepath.Join(file, "sub")).Set("k"); err == nil || !strings.Contains(err.Error(), "creating credentials directory") {
		t.Errorf("err = %v", err)
	}
}

func TestFileKeyring_SetCreatesPrivateFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	kr := NewFileKeyringForWorkspace(dir, "acme")
	if err := kr.Set("secret"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "credentials-acme"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials mode = %o, want 600", perm)
	}
	dinfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dinfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %o, want 700", perm)
	}
}

func TestFileKeyring_GetReturnsStoredBytesVerbatim(t *testing.T) {
	// Get does not trim; callers that hand-edit the file with a trailing
	// newline get the newline back in the key.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "credentials"), []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := NewFileKeyring(dir).Get()
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc\n" {
		t.Errorf("Get = %q", got)
	}
}

func TestFileKeyring_RemoveError(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory under the credentials name cannot be removed.
	if err := os.MkdirAll(filepath.Join(dir, "credentials", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := NewFileKeyring(dir).Remove(); err == nil {
		t.Error("expected an error")
	}
}

// BUG: workspace names are used verbatim in the credentials filename, so a
// name containing path separators escapes the config directory.
func TestFileKeyringForWorkspace_StaysInsideDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cfg")
	if err := NewFileKeyringForWorkspace(dir, "x/../../escaped").Set("k"); err != nil {
		return // rejecting the name is also acceptable
	}
	if _, err := os.Stat(filepath.Join(root, "escaped")); err == nil {
		t.Error("credentials were written outside the config directory")
	}
}

func TestResolveAPIKey_PropagatesBackendError(t *testing.T) {
	t.Setenv("PEPPOL_API_KEY", "")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "credentials"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveAPIKey(NewFileKeyring(dir)); err == nil {
		t.Error("expected the backend error")
	}
}
