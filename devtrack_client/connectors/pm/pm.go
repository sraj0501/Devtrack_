// Package pm is a thin platform-agnostic facade over the github/gitlab/azure
// connectors so the commit flow and ticket picker can list and comment on
// tickets without knowing per-platform types.
//
// API keys / secrets stay in .env (GITHUB_TOKEN, GITLAB_PAT, AZURE_DEVOPS_PAT).
// All other config (org, project, username, api_url) comes from workspaces.yaml.
package pm

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/gitlab"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Ticket is a platform-agnostic view of a PM issue / work item.
type Ticket struct {
	Platform string `json:"platform"` // "github" | "gitlab" | "azure"
	Number   int    `json:"number"`
	ID       string `json:"id"`   // e.g. "#42", "AB#1234"
	Title    string `json:"title"`
	Body     string `json:"body"`
	State    string `json:"state"`
	URL      string `json:"url"`
	Repo     string `json:"repo"` // "owner/repo" (github) or "group/project" (gitlab)
}

// SupportedPlatform reports whether the platform has a Go connector.
func SupportedPlatform(platform string) bool {
	switch strings.ToLower(platform) {
	case "github", "gitlab", "azure":
		return true
	}
	return false
}

// ── Client constructors ────────────────────────────────────────────────────────

// NewGitHubClient builds a GitHub client from workspace config + GITHUB_TOKEN env.
func NewGitHubClient(ws *config.WorkspaceConfig) (*github.Client, error) {
	apiURL := ""
	if ws != nil {
		apiURL = ws.PMAPIURL
	}
	return github.NewClient(apiURL)
}

// NewGitLabClient builds a GitLab client from workspace config + GITLAB_PAT env.
func NewGitLabClient(ws *config.WorkspaceConfig) (*gitlab.Client, error) {
	apiURL := ""
	if ws != nil {
		apiURL = ws.PMAPIURL
	}
	return gitlab.NewClient(apiURL)
}

// NewAzureClient builds an Azure DevOps client from workspace config + AZURE_DEVOPS_PAT env.
func NewAzureClient(ws *config.WorkspaceConfig) (*azure.Client, error) {
	org, project, apiURL := "", "", ""
	if ws != nil {
		org = ws.PMOrg
		project = ws.PMProject
		apiURL = ws.PMAPIURL
	}
	return azure.NewClient(org, project, apiURL)
}

// ── Ticket operations ─────────────────────────────────────────────────────────

// ListOpenTickets returns the open tickets assigned to the user for the
// workspace's PM platform. Returns (nil, nil) for unsupported/none platforms.
func ListOpenTickets(ws *config.WorkspaceConfig) ([]Ticket, error) {
	platform := ""
	if ws != nil {
		platform = strings.ToLower(ws.PMPlatform)
	}

	username := ""
	if ws != nil {
		username = ws.PMUsername
	}

	switch platform {
	case "github":
		c, err := NewGitHubClient(ws)
		if err != nil {
			return nil, err
		}
		issues, err := c.ListIssues(username)
		if err != nil {
			return nil, err
		}
		out := make([]Ticket, 0, len(issues))
		for _, i := range issues {
			out = append(out, Ticket{
				Platform: "github", Number: i.Number, ID: fmt.Sprintf("#%d", i.Number),
				Title: i.Title, Body: i.Body, State: i.State, URL: i.URL, Repo: i.Repo,
			})
		}
		return out, nil

	case "gitlab":
		c, err := NewGitLabClient(ws)
		if err != nil {
			return nil, err
		}
		issues, err := c.ListIssues(username)
		if err != nil {
			return nil, err
		}
		out := make([]Ticket, 0, len(issues))
		for _, i := range issues {
			out = append(out, Ticket{
				Platform: "gitlab", Number: i.IID, ID: fmt.Sprintf("#%d", i.IID),
				Title: i.Title, Body: i.Body, State: i.State, URL: i.URL, Repo: i.Repo,
			})
		}
		return out, nil

	case "azure":
		c, err := NewAzureClient(ws)
		if err != nil {
			return nil, err
		}
		items, err := c.ListWorkItems()
		if err != nil {
			return nil, err
		}
		out := make([]Ticket, 0, len(items))
		for _, w := range items {
			body := ""
			if v, ok := w.Fields["System.Description"]; ok {
				if s, ok := v.(string); ok {
					body = s
				}
			}
			out = append(out, Ticket{
				Platform: "azure", Number: w.ID, ID: fmt.Sprintf("AB#%d", w.ID),
				Title: w.Title(), Body: body, State: w.State(), URL: w.WebURL,
			})
		}
		return out, nil
	}

	return nil, nil
}

// CreateTicket opens a new ticket on the workspace's PM platform.
func CreateTicket(ws *config.WorkspaceConfig, repoPath, title, body string) (Ticket, error) {
	platform := ""
	if ws != nil {
		platform = strings.ToLower(ws.PMPlatform)
	}

	milestone := 0
	if ws != nil {
		milestone = ws.PMMilestone
	}

	switch platform {
	case "github":
		repo := repoForWorkspace(ws, repoPath)
		if repo == "" {
			return Ticket{}, fmt.Errorf("github: cannot determine owner/repo (set pm_project in workspaces.yaml)")
		}
		c, err := NewGitHubClient(ws)
		if err != nil {
			return Ticket{}, err
		}
		num, url, err := c.CreateIssue(repo, title, body, milestone)
		if err != nil {
			return Ticket{}, err
		}
		return Ticket{
			Platform: "github", Number: num, ID: fmt.Sprintf("#%d", num),
			Title: title, Body: body, State: "open", URL: url, Repo: repo,
		}, nil

	case "gitlab":
		repo := repoForWorkspace(ws, repoPath)
		if repo == "" {
			return Ticket{}, fmt.Errorf("gitlab: cannot determine group/project (set pm_project in workspaces.yaml)")
		}
		c, err := NewGitLabClient(ws)
		if err != nil {
			return Ticket{}, err
		}
		iid, url, err := c.CreateIssue(repo, title, body, milestone)
		if err != nil {
			return Ticket{}, err
		}
		return Ticket{
			Platform: "gitlab", Number: iid, ID: fmt.Sprintf("#%d", iid),
			Title: title, Body: body, State: "opened", URL: url, Repo: repo,
		}, nil

	case "azure":
		c, err := NewAzureClient(ws)
		if err != nil {
			return Ticket{}, err
		}
		id, url, err := c.CreateWorkItem(title, body, "")
		if err != nil {
			return Ticket{}, err
		}
		return Ticket{
			Platform: "azure", Number: id, ID: fmt.Sprintf("AB#%d", id),
			Title: title, Body: body, State: "New", URL: url,
		}, nil
	}

	return Ticket{}, fmt.Errorf("unsupported PM platform: %q", platform)
}

// AddComment posts body as a comment/note on the ticket.
func AddComment(ws *config.WorkspaceConfig, t Ticket, body string) error {
	switch t.Platform {
	case "github":
		c, err := NewGitHubClient(ws)
		if err != nil {
			return err
		}
		return c.AddIssueComment(t.Repo, t.Number, body)
	case "gitlab":
		c, err := NewGitLabClient(ws)
		if err != nil {
			return err
		}
		return c.AddIssueNote(t.Repo, t.Number, body)
	case "azure":
		c, err := NewAzureClient(ws)
		if err != nil {
			return err
		}
		return c.AddWorkItemComment(t.Number, body)
	}
	return fmt.Errorf("unsupported PM platform: %q", t.Platform)
}

// repoForWorkspace returns "owner/repo" or "group/project" for ticket creation.
// Uses pm_project from the workspace config first, then falls back to parsing
// the repo's git origin remote.
func repoForWorkspace(ws *config.WorkspaceConfig, repoPath string) string {
	if ws != nil && strings.TrimSpace(ws.PMProject) != "" {
		return strings.TrimSpace(ws.PMProject)
	}
	return parseRemoteRepo(repoPath)
}

// parseRemoteRepo reads origin's URL for repoPath and extracts owner/repo.
func parseRemoteRepo(repoPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseOwnerRepo(strings.TrimSpace(string(out)))
}

// parseOwnerRepo extracts "owner/repo" from a git remote URL (SSH or HTTPS).
func parseOwnerRepo(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	if i := strings.Index(remote, "://"); i >= 0 {
		rest := remote[i+3:]
		if s := strings.IndexByte(rest, '/'); s >= 0 {
			return strings.Trim(rest[s+1:], "/")
		}
		return ""
	}
	if i := strings.LastIndex(remote, ":"); i >= 0 {
		return strings.Trim(remote[i+1:], "/")
	}
	return ""
}

