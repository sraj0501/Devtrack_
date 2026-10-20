package infra

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

func TestWorkspaceIdentityIncludesTicketContract(t *testing.T) {
	base := config.WorkspaceConfig{Name: "repo", Path: "/repo", TicketKey: "PROJ"}
	for _, change := range []func(*config.WorkspaceConfig){
		func(ws *config.WorkspaceConfig) { ws.TicketKey = "GH" },
		func(ws *config.WorkspaceConfig) { ws.TicketPattern = `^work/(?P<ticket>PROJ-[1-9][0-9]*)$` },
		func(ws *config.WorkspaceConfig) { ws.PMPlatform = "github" },
	} {
		changed := base
		change(&changed)
		if workspaceKey(base) == workspaceKey(changed) {
			t.Fatal("ticket change would reuse stale monitor")
		}
	}
}

func TestReloadRetainsValidContractAndRestartsChangedWorkspace(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WORKSPACES_FILE", filepath.Join(root, "workspaces.yaml"))
	repo := filepath.Join(root, "repo")
	if _, err := gogit.PlainInit(repo, false); err != nil {
		t.Fatal(err)
	}
	cfg := config.WorkspacesConfig{Version: "1", Workspaces: []config.WorkspaceConfig{{Name: "test", Path: repo, Enabled: true, TicketKey: "PROJ"}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	im := &IntegratedMonitor{}
	im.ReloadWorkspaces()
	if len(im.workspaceMonitors) != 1 {
		t.Fatal("initial monitor missing")
	}
	defer func() {
		for _, wm := range im.workspaceMonitors {
			wm.gitMonitor.Stop()
		}
	}()
	original := im.workspaceMonitors[0]
	if err := os.WriteFile(config.GetWorkspacesFilePath(), []byte("version: '1'\nworkspaces:\n  - name: test\n    ticket_pattern: '[broken'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	im.ReloadWorkspaces()
	if im.workspaceMonitors[0] != original {
		t.Fatal("invalid reload discarded last valid monitor")
	}
	cfg.Workspaces[0].TicketPattern = `^work/(?P<ticket>PROJ-[1-9][0-9]*)$`
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	im.ReloadWorkspaces()
	if len(im.workspaceMonitors) != 1 || im.workspaceMonitors[0] == original || im.workspaceMonitors[0].ticketPattern != cfg.Workspaces[0].TicketPattern {
		t.Fatal("valid pattern change did not replace monitor")
	}
	updated := im.workspaceMonitors[0]
	im.ReloadWorkspaces()
	if im.workspaceMonitors[0] != updated {
		t.Fatal("unchanged config unnecessarily restarted monitor")
	}
}
