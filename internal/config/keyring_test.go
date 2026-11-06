package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileKeyring_SetGetRemove(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)

	// Initially empty
	key, err := kr.Get()
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}

	// Set
	if err := kr.Set("sk-test-12345"); err != nil {
		t.Fatalf("set error: %v", err)
	}

	// Get
	key, err = kr.Get()
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if key != "sk-test-12345" {
		t.Errorf("expected 'sk-test-12345', got %q", key)
	}

	// Remove
	if err := kr.Remove(); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	key, err = kr.Get()
	if err != nil {
		t.Fatalf("get error after remove: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key after remove, got %q", key)
	}
}

func TestFileKeyring_RemoveNonexistent(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	if err := kr.Remove(); err != nil {
		t.Errorf("remove nonexistent should not error, got %v", err)
	}
}

func TestResolveAPIKeySource_EnvVarPriority(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	kr.Set("keyring-key")

	t.Setenv("PEPPOL_API_KEY", "env-key")

	key, source, err := ResolveAPIKeySource(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "env-key" || source != KeySourceEnv {
		t.Errorf("got (%q, %v), want env-key from the env", key, source)
	}
}

func TestResolveAPIKeySource_FallsBackToKeyring(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	kr.Set("keyring-key")

	os.Unsetenv("PEPPOL_API_KEY")

	key, source, err := ResolveAPIKeySource(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "keyring-key" || source != KeySourceKeyring {
		t.Errorf("got (%q, %v), want keyring-key from the keyring", key, source)
	}
}

func TestResolveAPIKeySource_EmptyWhenNone(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	os.Unsetenv("PEPPOL_API_KEY")

	key, source, err := ResolveAPIKeySource(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "" || source != KeySourceNone {
		t.Errorf("got (%q, %v), want no key", key, source)
	}
}

func TestFileKeyringForWorkspace_Isolation(t *testing.T) {
	dir := t.TempDir()

	krA := NewFileKeyringForWorkspace(dir, "alpha")
	krB := NewFileKeyringForWorkspace(dir, "beta")

	if err := krA.Set("key-alpha"); err != nil {
		t.Fatalf("set alpha: %v", err)
	}
	if err := krB.Set("key-beta"); err != nil {
		t.Fatalf("set beta: %v", err)
	}

	keyA, _ := krA.Get()
	keyB, _ := krB.Get()

	if keyA != "key-alpha" {
		t.Errorf("expected 'key-alpha', got %q", keyA)
	}
	if keyB != "key-beta" {
		t.Errorf("expected 'key-beta', got %q", keyB)
	}

	// Remove one, other unaffected.
	if err := krA.Remove(); err != nil {
		t.Fatalf("remove alpha: %v", err)
	}
	keyA, _ = krA.Get()
	keyB, _ = krB.Get()
	if keyA != "" {
		t.Errorf("alpha should be empty after remove, got %q", keyA)
	}
	if keyB != "key-beta" {
		t.Errorf("beta should be unaffected, got %q", keyB)
	}
}

func TestFileKeyringForWorkspace_IndependentFromDefault(t *testing.T) {
	dir := t.TempDir()

	krDefault := NewFileKeyring(dir)
	krWs := NewFileKeyringForWorkspace(dir, "myco")

	krDefault.Set("default-key")
	krWs.Set("workspace-key")

	keyD, _ := krDefault.Get()
	keyW, _ := krWs.Get()

	if keyD != "default-key" {
		t.Errorf("expected 'default-key', got %q", keyD)
	}
	if keyW != "workspace-key" {
		t.Errorf("expected 'workspace-key', got %q", keyW)
	}
}

func TestValidateWorkspaceName(t *testing.T) {
	valid := []string{
		"default", "prod", "my-company-b-v", "Acme_Corp", "v1.2", "a",
		"..a", "a..b", ".hidden", strings.Repeat("a", MaxWorkspaceNameLen),
	}
	for _, name := range valid {
		if err := ValidateWorkspaceName(name); err != nil {
			t.Errorf("ValidateWorkspaceName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{
		"", ".", "..", "x/../../escaped", "../escaped", "a/b", `a\b`,
		"has space", "tab\tname", "nul\x00", "caf\u00e9", "semi;colon",
		strings.Repeat("a", MaxWorkspaceNameLen+1),
	}
	for _, name := range invalid {
		if err := ValidateWorkspaceName(name); err == nil {
			t.Errorf("ValidateWorkspaceName(%q) = nil, want error", name)
		}
	}
}

func TestMaxWorkspaceNameLen_FitsFilenameLimit(t *testing.T) {
	if n := len(credsFile + "-" + strings.Repeat("a", MaxWorkspaceNameLen)); n > 255 {
		t.Errorf("credentials filename length %d exceeds 255", n)
	}
}

func TestFileKeyringForWorkspace_RejectsEscapingNames(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cfg")
	for _, name := range []string{"", "x/../../escaped", "a/b", "../x", "nul\x00"} {
		kr := NewFileKeyringForWorkspace(dir, name)
		if err := kr.Set("k"); err == nil {
			t.Errorf("Set with workspace %q succeeded, want error", name)
		}
		if _, err := kr.Get(); err == nil {
			t.Errorf("Get with workspace %q succeeded, want error", name)
		}
		if err := kr.Remove(); err == nil {
			t.Errorf("Remove with workspace %q succeeded, want error", name)
		}
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.Name() != "cfg" {
			t.Errorf("unexpected file outside config dir: %s", e.Name())
		}
	}
}

func TestFileKeyringForWorkspace_AllowsLegacyNamesInsideDir(t *testing.T) {
	// Names created before validation existed (e.g. with spaces) keep working
	// as long as the file stays inside the config dir.
	dir := t.TempDir()
	kr := NewFileKeyringForWorkspace(dir, "My Prod")
	if err := kr.Set("k"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, err := kr.Get(); err != nil || got != "k" {
		t.Errorf("Get = %q, %v; want k, nil", got, err)
	}
}

func TestResolveAPIKeySource(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		stored     string
		wantKey    string
		wantSource KeySource
	}{
		{"env wins over keyring", "env-key", "keyring-key", "env-key", KeySourceEnv},
		{"whitespace env wins over keyring", "    ", "keyring-key", "    ", KeySourceEnv},
		{"keyring when env unset", "", "keyring-key", "keyring-key", KeySourceKeyring},
		{"env with empty keyring", "env-key", "", "env-key", KeySourceEnv},
		{"none", "", "", "", KeySourceNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kr := NewFileKeyring(t.TempDir())
			if tt.stored != "" {
				if err := kr.Set(tt.stored); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PEPPOL_API_KEY", tt.env)

			key, source, err := ResolveAPIKeySource(kr)
			if err != nil {
				t.Fatalf("resolve error: %v", err)
			}
			if key != tt.wantKey || source != tt.wantSource {
				t.Errorf("got (%q, %v), want (%q, %v)", key, source, tt.wantKey, tt.wantSource)
			}
		})
	}
}
