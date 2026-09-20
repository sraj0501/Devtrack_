package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspacesConfigCreatesEmptyAuthoritativeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "workspaces.yaml")
	t.Setenv("WORKSPACES_FILE", path)
	t.Setenv("DEVTRACK_WORKSPACE", filepath.Join(t.TempDir(), "legacy-repo"))

	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		t.Fatalf("LoadWorkspacesConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadWorkspacesConfig() returned nil config")
	}
	if cfg.Version != "1" {
		t.Fatalf("Version = %q, want 1", cfg.Version)
	}
	if len(cfg.Workspaces) != 0 {
		t.Fatalf("Workspaces = %#v, want empty", cfg.Workspaces)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("workspaces file was not created: %v", err)
	}
	if string(data) != "version: \"1\"\nworkspaces: []\n" {
		t.Fatalf("workspaces file = %q, want empty workspace document", data)
	}
}
