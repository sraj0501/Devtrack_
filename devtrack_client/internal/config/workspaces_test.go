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

func TestLoadWorkspacesConfigRejectsInvalidTicketContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspaces.yaml")
	t.Setenv("WORKSPACES_FILE", path)
	data := []byte("version: \"1\"\nworkspaces:\n  - name: test\n    path: /repo\n    enabled: true\n    ticket_pattern: '(?P<ticket>PROJ-[0-9]+)'\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWorkspacesConfig(); err == nil {
		t.Fatal("LoadWorkspacesConfig() succeeded with an unanchored ticket_pattern")
	}
}

func TestWorkspacesSaveRejectsInvalidTicketKey(t *testing.T) {
	t.Setenv("WORKSPACES_FILE", filepath.Join(t.TempDir(), "workspaces.yaml"))
	cfg := &WorkspacesConfig{Version: "1", Workspaces: []WorkspaceConfig{{
		Name: "test", Path: "/repo", Enabled: true, TicketKey: "proj",
	}}}
	if err := cfg.Save(); err == nil {
		t.Fatal("Save() succeeded with a lowercase ticket_key")
	}
}
