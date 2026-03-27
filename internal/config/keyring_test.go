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
