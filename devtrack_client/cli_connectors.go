package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	azureconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	githubconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	gitlabconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/gitlab"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/pm"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// workspaceFor finds the workspace for the given platform from workspaces.yaml.
// It first tries to match the current directory; falls back to the first enabled
// workspace for that platform. Returns an error if none is found.
func workspaceFor(platform string) (*config.WorkspaceConfig, error) {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot load workspaces.yaml: %w", err)
	}
	if wsCfg == nil || len(wsCfg.Workspaces) == 0 {
		return nil, fmt.Errorf("workspaces.yaml not found or empty — add a workspace entry for %s", platform)
	}

	cwd, _ := os.Getwd()

	// Prefer a workspace that matches the current directory AND the platform.
	for i := range wsCfg.Workspaces {
		ws := &wsCfg.Workspaces[i]
		if !ws.Enabled {
			continue
		}
		if platform != "" && !strings.EqualFold(ws.PMPlatform, platform) {
			continue
		}
		if strings.HasPrefix(cwd, ws.Path) {
			return ws, nil
		}
	}

	// Fall back to first enabled workspace for this platform.
	for i := range wsCfg.Workspaces {
		ws := &wsCfg.Workspaces[i]
		if ws.Enabled && strings.EqualFold(ws.PMPlatform, platform) {
			return ws, nil
		}
	}

	return nil, fmt.Errorf("no enabled %s workspace found in workspaces.yaml", platform)
}

// ── Azure ─────────────────────────────────────────────────────────────────────

func (cli *CLI) handleAzureCheck() error {
	ws, err := workspaceFor("azure")
	if err != nil {
		return err
	}
	c, err := pm.NewAzureClient(ws)
	if err != nil {
		return err
	}
	return c.Check()
}

func (cli *CLI) handleAzureList() error {
	ws, err := workspaceFor("azure")
	if err != nil {
		return err
	}
	c, err := pm.NewAzureClient(ws)
	if err != nil {
		return err
	}
	items, err := c.ListWorkItems()
	if err != nil {
		return fmt.Errorf("azure list: %w", err)
	}
	if len(items) == 0 {
		fmt.Println("No open work items assigned to you.")
		return nil
	}
	for _, item := range items {
		fmt.Printf("AB#%d  [%s] (%s)  %s\n  %s\n\n",
			item.ID, item.State(), item.WorkItemType(), item.Title(), item.WebURL)
	}
	return nil
}

func (cli *CLI) handleAzureSync() error {
	ws, err := workspaceFor("azure")
	if err != nil {
		return err
	}
	c, err := pm.NewAzureClient(ws)
	if err != nil {
		return err
	}
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("azure sync: open database: %w", err)
	}
	defer db.Close()
	if err := c.Sync(db.DB()); err != nil {
		return err
	}
	if err := pushToServer(trigger.NewHTTPTriggerClient(), "azure", ws.Name, false,
		time.Now().UTC().Format(time.RFC3339), readAzureCached(db.DB())); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  Server push failed — local sync complete, server cache not updated.\n    Check 'devtrack logs' for details.\n")
	}
	return nil
}

func (cli *CLI) handleAzureView() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack azure-view <work-item-id>")
		return fmt.Errorf("missing work item ID")
	}
	id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return fmt.Errorf("azure-view: invalid work item ID %q: %w", os.Args[2], err)
	}
	ws, err := workspaceFor("azure")
	if err != nil {
		return err
	}
	c, err := pm.NewAzureClient(ws)
	if err != nil {
		return err
	}
	item, err := c.ViewWorkItem(id)
	if err != nil {
		return fmt.Errorf("azure view: %w", err)
	}
	fmt.Print(azureconn.FormatWorkItem(item))
	return nil
}

// ── GitLab ────────────────────────────────────────────────────────────────────

func (cli *CLI) handleGitLabCheck() error {
	ws, err := workspaceFor("gitlab")
	if err != nil {
		return err
	}
	c, err := pm.NewGitLabClient(ws)
	if err != nil {
		return err
	}
	return c.Check()
}

func (cli *CLI) handleGitLabList() error {
	ws, err := workspaceFor("gitlab")
	if err != nil {
		return err
	}
	c, err := pm.NewGitLabClient(ws)
	if err != nil {
		return err
	}
	username := ""
	if ws != nil {
		username = ws.PMUsername
	}
	issues, err := c.ListIssues(username)
	if err != nil {
		return fmt.Errorf("gitlab list: %w", err)
	}
	if len(issues) == 0 {
		fmt.Println("No open issues assigned to you.")
		return nil
	}
	for _, iss := range issues {
		fmt.Printf("#%d  [%s]  %s\n  %s\n\n", iss.IID, iss.State, iss.Title, iss.URL)
	}
	return nil
}

func (cli *CLI) handleGitLabSync() error {
	ws, err := workspaceFor("gitlab")
	if err != nil {
		return err
	}
	c, err := pm.NewGitLabClient(ws)
	if err != nil {
		return err
	}
	username := ""
	if ws != nil {
		username = ws.PMUsername
	}
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("gitlab sync: open database: %w", err)
	}
	defer db.Close()
	if err := c.Sync(db.DB(), username); err != nil {
		return err
	}
	repo := ""
	if ws != nil {
		repo = ws.PMProject
	}
	if err := pushToServer(trigger.NewHTTPTriggerClient(), "gitlab", ws.Name, false,
		time.Now().UTC().Format(time.RFC3339), readGitLabCached(db.DB(), repo)); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  Server push failed — local sync complete, server cache not updated.\n    Check 'devtrack logs' for details.\n")
	}
	return nil
}

func (cli *CLI) handleGitLabView() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: devtrack gitlab-view <project_id_or_path> <issue_iid>")
		return fmt.Errorf("missing project_id and/or issue_iid")
	}
	projectPath := os.Args[2]
	iid, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return fmt.Errorf("gitlab-view: invalid issue IID %q: %w", os.Args[3], err)
	}
	ws, err := workspaceFor("gitlab")
	if err != nil {
		return err
	}
	c, err := pm.NewGitLabClient(ws)
	if err != nil {
		return err
	}
	issue, err := c.ViewIssue(projectPath, iid)
	if err != nil {
		return fmt.Errorf("gitlab view: %w", err)
	}
	fmt.Print(gitlabconn.FormatIssue(issue))
	return nil
}

// ── GitHub ────────────────────────────────────────────────────────────────────

func (cli *CLI) handleGitHubCheck() error {
	ws, err := workspaceFor("github")
	if err != nil {
		return err
	}
	c, err := pm.NewGitHubClient(ws)
	if err != nil {
		return err
	}
	return c.Check()
}

func (cli *CLI) handleGitHubList() error {
	ws, err := workspaceFor("github")
	if err != nil {
		return err
	}
	c, err := pm.NewGitHubClient(ws)
	if err != nil {
		return err
	}
	username := ""
	if ws != nil {
		username = ws.PMUsername
	}
	issues, err := c.ListIssues(username)
	if err != nil {
		return fmt.Errorf("github list: %w", err)
	}
	if len(issues) == 0 {
		fmt.Println("No open issues assigned to you.")
		return nil
	}
	for _, iss := range issues {
		fmt.Printf("#%d  [%s]  %s\n  %s\n  %s\n\n",
			iss.Number, iss.State, iss.Title, iss.Repo, iss.URL)
	}
	return nil
}

func (cli *CLI) handleGitHubSync() error {
	ws, err := workspaceFor("github")
	if err != nil {
		return err
	}
	c, err := pm.NewGitHubClient(ws)
	if err != nil {
		return err
	}
	username := ""
	if ws != nil {
		username = ws.PMUsername
	}
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("github sync: open database: %w", err)
	}
	defer db.Close()
	if err := c.Sync(db.DB(), username); err != nil {
		return err
	}
	repo := ""
	if ws != nil {
		repo = ws.PMProject
	}
	if err := pushToServer(trigger.NewHTTPTriggerClient(), "github", ws.Name, false,
		time.Now().UTC().Format(time.RFC3339), readGitHubCached(db.DB(), repo)); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  Server push failed — local sync complete, server cache not updated.\n    Check 'devtrack logs' for details.\n")
	}
	return nil
}

// handleTicketSync syncs all configured PM platforms and pushes to the server.
// --force drops the server-side cache before reloading (clean slate).
func (cli *CLI) handleTicketSync() error {
	force := false
	for _, arg := range os.Args[2:] {
		if arg == "--force" {
			force = true
		}
	}

	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("ticket-sync: open database: %w", err)
	}
	defer db.Close()

	if force {
		fmt.Println("Force-refreshing ticket cache (drop + reload)...")
	} else {
		fmt.Println("Syncing tickets (update existing + add new)...")
	}

	// SyncAllTickets logs push failures internally. The local SQLite cache is
	// always updated regardless of whether Python is reachable.
	SyncAllTickets(db, force)

	fmt.Println("✓ Local cache updated.")
	if !trigger.NewHTTPTriggerClient().Ping() {
		fmt.Fprintln(os.Stderr, "⚠  Server unreachable — ticket data will be pushed on next successful connection.")
		fmt.Fprintln(os.Stderr, "    Check 'devtrack logs' for details.")
	} else {
		fmt.Println("✓ Server cache updated.")
	}
	return nil
}

// handleGitHubView shows details for a specific GitHub issue.
// Usage: devtrack github-view <owner/repo> <issue_number>
//
//	devtrack github-view <issue_number>  (uses pm_project from workspaces.yaml)
func (cli *CLI) handleGitHubView() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack github-view <owner/repo> <issue_number>")
		fmt.Println("   or: devtrack github-view <issue_number>  (uses pm_project from workspaces.yaml)")
		return fmt.Errorf("missing arguments")
	}

	ws, wsErr := workspaceFor("github")

	var owner, repo string
	var number int
	var err error

	if len(os.Args) >= 4 {
		parts := strings.SplitN(os.Args[2], "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("github-view: expected owner/repo, got %q", os.Args[2])
		}
		owner, repo = parts[0], parts[1]
		number, err = strconv.Atoi(os.Args[3])
	} else {
		// Single-arg form: issue number only — use pm_project from workspace.
		if wsErr != nil {
			return fmt.Errorf("github-view: %w — or specify owner/repo explicitly", wsErr)
		}
		if ws.PMProject == "" {
			return fmt.Errorf("github-view: set pm_project in workspaces.yaml, or use: devtrack github-view <owner/repo> <number>")
		}
		parts := strings.SplitN(ws.PMProject, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("github-view: pm_project must be owner/repo, got %q", ws.PMProject)
		}
		owner, repo = parts[0], parts[1]
		number, err = strconv.Atoi(os.Args[2])
	}
	if err != nil {
		return fmt.Errorf("github-view: invalid issue number: %w", err)
	}

	c, err := pm.NewGitHubClient(ws)
	if err != nil {
		return err
	}
	issue, err := c.ViewIssue(owner, repo, number)
	if err != nil {
		return fmt.Errorf("github view: %w", err)
	}
	fmt.Print(githubconn.FormatIssue(issue))
	return nil
}
