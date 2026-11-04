package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

// isolateHome points XDG_CONFIG_HOME and HOME at a temp dir and returns it.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

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

func TestLoadSave_RoundTripUnderConfigDir(t *testing.T) {
	root := isolateHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workspaces == nil || cfg.ActiveWorkspace != "" {
		t.Errorf("default config = %+v, want empty with a non-nil map", cfg)
	}

	if err := cfg.AddWorkspace("acme", Workspace{Name: "ACME NV"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "peppol-cli", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config.yaml mode = %o, want 600", perm)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveWorkspace != "acme" || got.Workspaces["acme"].Name != "ACME NV" {
		t.Errorf("loaded %+v", got)
	}
}

func TestLoad_ErrorPaths(t *testing.T) {
	t.Run("corrupt yaml", func(t *testing.T) {
		root := isolateHome(t)
		writeConfig(t, filepath.Join(root, "peppol-cli"), "active_workspace: [unterminated")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "parsing config") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("config path is a directory", func(t *testing.T) {
		root := isolateHome(t)
		if err := os.MkdirAll(filepath.Join(root, "peppol-cli", "config.yaml"), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "reading config") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no home", func(t *testing.T) {
		noHome(t)
		if _, err := Load(); err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("missing workspaces map", func(t *testing.T) {
		root := isolateHome(t)
		writeConfig(t, filepath.Join(root, "peppol-cli"), "active_workspace: solo\n")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Workspaces == nil {
			t.Error("Workspaces should be initialized so AddWorkspace does not panic")
		}
		if cfg.ActiveWorkspace != "solo" {
			t.Errorf("ActiveWorkspace = %q", cfg.ActiveWorkspace)
		}
	})
}

func TestSave_ErrorPaths(t *testing.T) {
	t.Run("no home", func(t *testing.T) {
		noHome(t)
		if err := Save(&Config{}); err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("config dir is a file", func(t *testing.T) {
		root := isolateHome(t)
		if err := os.WriteFile(filepath.Join(root, "peppol-cli"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Save(&Config{}); err == nil || !strings.Contains(err.Error(), "creating config directory") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("config file path is a directory", func(t *testing.T) {
		root := isolateHome(t)
		if err := os.MkdirAll(filepath.Join(root, "peppol-cli", "config.yaml"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := Save(&Config{}); err == nil || !strings.Contains(err.Error(), "writing config") {
			t.Errorf("err = %v", err)
		}
	})
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
	if os.Getenv("PEPPOL_SHOW_BUGS") == "" {
		t.Skip("BUG: keyring.go NewFileKeyringForWorkspace joins \"credentials-\"+workspace without validation")
	}
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

// --- OSKeyring ---
//
// These use keyring.ArrayKeyring (in memory), so the real system keychain is
// never touched.

func TestOSKeyring_GetSetRemoveWithInMemoryRing(t *testing.T) {
	k := &OSKeyring{ring: keyring.NewArrayKeyring(nil)}

	got, err := k.Get()
	if err != nil || got != "" {
		t.Fatalf("Get on empty ring = %q, %v; want empty, nil", got, err)
	}
	if err := k.Remove(); err != nil {
		t.Errorf("Remove on empty ring = %v, want nil", err)
	}
	if err := k.Set("secret"); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get(); err != nil || got != "secret" {
		t.Errorf("Get = %q, %v", got, err)
	}
	if err := k.Remove(); err != nil {
		t.Fatal(err)
	}
	if got, _ := k.Get(); got != "" {
		t.Errorf("Get after Remove = %q", got)
	}
}

type failingRing struct{ keyring.Keyring }

var errRing = errors.New("ring broken")

func (failingRing) Get(string) (keyring.Item, error) { return keyring.Item{}, errRing }
func (failingRing) Remove(string) error              { return errRing }

func TestOSKeyring_WrapsBackendErrors(t *testing.T) {
	k := &OSKeyring{ring: failingRing{keyring.NewArrayKeyring(nil)}}
	if _, err := k.Get(); !errors.Is(err, errRing) || !strings.Contains(err.Error(), "reading from keyring") {
		t.Errorf("Get err = %v", err)
	}
	if err := k.Remove(); !errors.Is(err, errRing) {
		t.Errorf("Remove err = %v", err)
	}
}

func TestNewOSKeyring_NoHomeIsError(t *testing.T) {
	noHome(t)
	if _, err := NewOSKeyring(); err == nil {
		t.Error("expected an error when the config dir cannot be determined")
	}
}

func TestNewOSKeyring_FileBackendOnly(t *testing.T) {
	for _, b := range keyring.AvailableBackends() {
		if b != keyring.FileBackend {
			t.Skipf("system keyring backend %q is available; not touching it", b)
		}
	}
	isolateHome(t)
	k, err := NewOSKeyring()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get(); err != nil || got != "" {
		t.Errorf("Get = %q, %v", got, err)
	}
}
