// Package pm is a thin platform-agnostic facade over the github/gitlab/azure
// connectors so the commit flow and ticket picker can list and comment on
// tickets without knowing per-platform types.
package pm

import (
	"fmt"
	"os"
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
