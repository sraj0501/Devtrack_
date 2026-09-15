package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoPathDoesNotUseLegacyEnvWorkspace(t *testing.T) {
	workspacesPath := filepath.Join(t.TempDir(), "workspaces.yaml")
	t.Setenv("WORKSPACES_FILE", workspacesPath)
	t.Setenv("DEVTRACK_WORKSPACE", t.TempDir())

	_, err := resolveRepoPath()
	if err == nil {
		t.Fatal("resolveRepoPath() succeeded with only DEVTRACK_WORKSPACE configured")
	}
	if !strings.Contains(err.Error(), "no enabled workspaces configured") {
		t.Fatalf("resolveRepoPath() error = %q, want workspace guidance", err)
	}

	if _, statErr := os.Stat(workspacesPath); statErr != nil {
		t.Fatalf("empty workspaces file was not created: %v", statErr)
	}
}
