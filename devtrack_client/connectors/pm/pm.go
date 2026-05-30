// Package pm is a thin platform-agnostic facade over the github/gitlab/azure
// connectors so the commit flow and ticket picker can list and comment on
// tickets without knowing per-platform types.
package pm

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/gitlab"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Ticket is a platform-agnostic view of a PM issue / work item.
// It is JSON-serialisable so it can be stashed in the pm_update_queue payload.
type Ticket struct {
	Platform string `json:"platform"` // "github" | "gitlab" | "azure"
	Number   int    `json:"number"`   // issue number / IID / work-item id
	ID       string `json:"id"`       // display + commit ref, e.g. "#42", "AB#1234"
	Title    string `json:"title"`
	Body     string `json:"body"`
	State    string `json:"state"`
	URL      string `json:"url"`
	Repo     string `json:"repo"` // "owner/repo" (github) or "group/project" (gitlab)
}

// SupportedPlatform reports whether the platform has a Go connector with
// list + comment support today (Jira is server-side only).
func SupportedPlatform(platform string) bool {
	switch strings.ToLower(platform) {
	case "github", "gitlab", "azure":
		return true
	}
	return false
}

// ListOpenTickets returns the open tickets assigned to the user for the
// workspace's PM platform. Returns (nil, nil) for unsupported/none platforms.
func ListOpenTickets(ws *config.WorkspaceConfig) ([]Ticket, error) {
	platform := ""
	if ws != nil {
		platform = strings.ToLower(ws.PMPlatform)
	}

	switch platform {
	case "github":
		issues, err := github.ListIssues(os.Getenv("GITHUB_TOKEN"), os.Getenv("GITHUB_USERNAME"))
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
		issues, err := gitlab.ListIssues()
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
		items, err := azure.ListWorkItems()
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

// CreateTicket opens a new ticket on the workspace's PM platform and returns
// the created Ticket. For github/gitlab the target repo is taken from the
// workspace's pm_project (falling back to the repo's origin remote).
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
			return Ticket{}, fmt.Errorf("github: cannot determine owner/repo (set pm_project or an origin remote)")
		}
		num, url, err := github.CreateIssue(repo, title, body, milestone)
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
			return Ticket{}, fmt.Errorf("gitlab: cannot determine group/project (set pm_project or an origin remote)")
		}
		iid, url, err := gitlab.CreateIssue(repo, title, body, milestone)
		if err != nil {
			return Ticket{}, err
		}
		return Ticket{
			Platform: "gitlab", Number: iid, ID: fmt.Sprintf("#%d", iid),
			Title: title, Body: body, State: "opened", URL: url, Repo: repo,
		}, nil

	case "azure":
		id, url, err := azure.CreateWorkItem(title, body, "")
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

// repoForWorkspace returns the github "owner/repo" / gitlab "group/project"
// for ticket creation: the workspace's pm_project if set, else parsed from the
// repo's origin remote.
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

// parseOwnerRepo extracts "owner/repo" (or "group/sub/project") from a git
// remote URL in either SSH (git@host:owner/repo.git) or HTTPS
// (https://host/owner/repo.git) form.
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

// AddComment posts body as a comment/note on the ticket.
func AddComment(t Ticket, body string) error {
	switch t.Platform {
	case "github":
		return github.AddIssueComment(t.Repo, t.Number, body)
	case "gitlab":
		return gitlab.AddIssueNote(t.Repo, t.Number, body)
	case "azure":
		return azure.AddWorkItemComment(t.Number, body)
	}
	return fmt.Errorf("unsupported PM platform: %q", t.Platform)
}
