package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveWorkspace != "" {
		t.Errorf("expected empty active workspace, got %q", cfg.ActiveWorkspace)
	}
	if cfg.Workspaces == nil {
		t.Error("expected non-nil workspaces map")
	}
}

func TestSaveToAndLoadFrom(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		ActiveWorkspace: "default",
		Workspaces: map[string]Workspace{
			"default": {Name: "My Company"},
		},
	}

	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}

	loaded, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.ActiveWorkspace != "default" {
		t.Errorf("expected active workspace 'default', got %q", loaded.ActiveWorkspace)
	}
	ws, ok := loaded.Workspaces["default"]
	if !ok {
		t.Fatal("expected 'default' workspace")
	}
	if ws.Name != "My Company" {
		t.Errorf("expected workspace name 'My Company', got %q", ws.Name)
	}
}

func TestSaveTo_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cfg := &Config{ActiveWorkspace: "test"}

	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.ActiveWorkspace != "test" {
		t.Errorf("expected 'test', got %q", loaded.ActiveWorkspace)
	}
}
