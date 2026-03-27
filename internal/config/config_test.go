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

func TestAddWorkspace(t *testing.T) {
	cfg := &Config{Workspaces: make(map[string]Workspace)}

	// First workspace becomes active.
	if err := cfg.AddWorkspace("alpha", Workspace{Name: "Alpha Co"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveWorkspace != "alpha" {
		t.Errorf("expected active 'alpha', got %q", cfg.ActiveWorkspace)
	}

	// Second workspace does not change active.
	if err := cfg.AddWorkspace("beta", Workspace{Name: "Beta Co"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveWorkspace != "alpha" {
		t.Errorf("active should still be 'alpha', got %q", cfg.ActiveWorkspace)
	}

	// Duplicate returns error.
	if err := cfg.AddWorkspace("alpha", Workspace{Name: "Alpha Again"}); err == nil {
		t.Fatal("expected error for duplicate workspace")
	}
}

func TestRemoveWorkspace(t *testing.T) {
	cfg := &Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}

	// Cannot remove active when others exist.
	if err := cfg.RemoveWorkspace("alpha"); err == nil {
		t.Fatal("expected error when removing active workspace")
	}

	// Can remove non-active.
	if err := cfg.RemoveWorkspace("beta"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Workspaces["beta"]; ok {
		t.Error("beta should have been removed")
	}

	// Can remove last workspace (which is active).
	if err := cfg.RemoveWorkspace("alpha"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveWorkspace != "" {
		t.Errorf("active should be cleared, got %q", cfg.ActiveWorkspace)
	}

	// Non-existent returns error.
	if err := cfg.RemoveWorkspace("gamma"); err == nil {
		t.Fatal("expected error for non-existent workspace")
	}
}

func TestSetActiveWorkspace(t *testing.T) {
	cfg := &Config{
		ActiveWorkspace: "alpha",
		Workspaces: map[string]Workspace{
			"alpha": {Name: "Alpha Co"},
			"beta":  {Name: "Beta Co"},
		},
	}

	if err := cfg.SetActiveWorkspace("beta"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveWorkspace != "beta" {
		t.Errorf("expected 'beta', got %q", cfg.ActiveWorkspace)
	}

	if err := cfg.SetActiveWorkspace("nonexistent"); err == nil {
		t.Fatal("expected error for non-existent workspace")
	}
}

func TestWorkspaceNames(t *testing.T) {
	cfg := &Config{
		Workspaces: map[string]Workspace{
			"charlie": {Name: "C"},
			"alpha":   {Name: "A"},
			"beta":    {Name: "B"},
		},
	}

	names := cfg.WorkspaceNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" || names[2] != "charlie" {
		t.Errorf("expected sorted names, got %v", names)
	}
}

func TestWorkspaceNames_Empty(t *testing.T) {
	cfg := &Config{Workspaces: make(map[string]Workspace)}
	if len(cfg.WorkspaceNames()) != 0 {
		t.Error("expected empty")
	}
}

func TestSaveAndLoadMultipleWorkspaces(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		ActiveWorkspace: "myco",
		Workspaces: map[string]Workspace{
			"myco":  {Name: "My Company"},
			"other": {Name: "Other Co"},
		},
	}

	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.ActiveWorkspace != "myco" {
		t.Errorf("expected active 'myco', got %q", loaded.ActiveWorkspace)
	}
	if len(loaded.Workspaces) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(loaded.Workspaces))
	}
}
