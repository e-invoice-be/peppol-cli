package config

import (
	"os"
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

func TestResolveAPIKey_EnvVarPriority(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	kr.Set("keyring-key")

	t.Setenv("PEPPOL_API_KEY", "env-key")

	key, err := ResolveAPIKey(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "env-key" {
		t.Errorf("expected 'env-key' (env var priority), got %q", key)
	}
}

func TestResolveAPIKey_FallsBackToKeyring(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	kr.Set("keyring-key")

	os.Unsetenv("PEPPOL_API_KEY")

	key, err := ResolveAPIKey(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "keyring-key" {
		t.Errorf("expected 'keyring-key', got %q", key)
	}
}

func TestResolveAPIKey_EmptyWhenNone(t *testing.T) {
	dir := t.TempDir()
	kr := NewFileKeyring(dir)
	os.Unsetenv("PEPPOL_API_KEY")

	key, err := ResolveAPIKey(kr)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
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
