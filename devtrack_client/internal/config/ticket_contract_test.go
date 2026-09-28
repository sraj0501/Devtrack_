package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceTicketDefaultsAndValidation(t *testing.T) {
	for platform, key := range map[string]string{"github": "GH", "gitlab": "GL", "azure": "ADO", "jira": "PROJ", "none": "PROJ"} {
		ws := WorkspaceConfig{PMPlatform: platform}
		if got := ws.TicketContract().Key; got != key {
			t.Fatalf("platform %s: key=%s", platform, got)
		}
		cfg := WorkspacesConfig{Workspaces: []WorkspaceConfig{ws}}
		if err := cfg.ValidateTickets(); err != nil {
			t.Fatal(err)
		}
	}
	cfg := WorkspacesConfig{Workspaces: []WorkspaceConfig{{Name: "bad", TicketPattern: `(?P<ticket>PROJ-\d+)`}}}
	if cfg.ValidateTickets() == nil {
		t.Fatal("substring pattern was accepted")
	}
}

func TestInvalidTicketConfigDoesNotOverwriteFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DEVTRACK_DATA_HOME", root)
	t.Setenv("WORKSPACES_FILE", filepath.Join(root, "workspaces.yaml"))
	cfg := WorkspacesConfig{Version: "1", Workspaces: []WorkspaceConfig{{Name: "test", PMPlatform: "github", Path: root, Enabled: true}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	path := GetWorkspacesFilePath()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "ticket_key: GH") {
		t.Fatalf("default not persisted: %s", before)
	}
	cfg.Workspaces[0].TicketPattern = `(?P<ticket>GH-\d+)`
	if cfg.Save() == nil {
		t.Fatal("invalid save succeeded")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("invalid configuration replaced valid file")
	}
	if err := os.WriteFile(path, []byte("version: '1'\nworkspaces:\n  - name: broken\n    ticket_pattern: '[invalid'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWorkspacesConfig(); err == nil {
		t.Fatal("invalid load silently fell back")
	}
}
