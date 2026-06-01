package main

// ticket_sync.go — sync all PM platform tickets to local SQLite and push a
// slim representation to the Python server for fuzzy/semantic matching.
//
// Data flow:
//   PM API → github_issues / azure_workitems / gitlab_issues (local SQLite)
//          → POST /trigger/ticket_sync (slim payload, Python ticket_cache)
//
// The two steps are intentionally separated: Go always keeps a full local
// copy; Python only receives what it needs for matching.

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	azureconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	githubconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	gitlabconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/gitlab"
	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/pm"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

const descriptionMaxLen = 500

func truncate(s string) string {
	if len(s) <= descriptionMaxLen {
		return s
	}
	return s[:descriptionMaxLen]
}

// SyncAllTickets syncs tickets for every enabled workspace and pushes the
// slim payload to the Python server.
//
// force=true  → Python drops all cached entries for the source then reloads
// force=false → Python upserts (add new + update existing, stale entries kept)
func SyncAllTickets(database *Database, force bool) {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		log.Printf("ticket-sync: no workspaces config — skipping")
		return
	}

	httpClient := trigger.NewHTTPTriggerClient()
	syncedAt := time.Now().UTC().Format(time.RFC3339)

	for i := range wsCfg.GetEnabledWorkspaces() {
		ws := wsCfg.GetEnabledWorkspaces()[i]
		switch strings.ToLower(ws.PMPlatform) {
		case "github":
			syncGitHub(database.DB(), &ws, httpClient, force, syncedAt)
		case "azure":
			syncAzure(database.DB(), &ws, httpClient, force, syncedAt)
		case "gitlab":
			syncGitLab(database.DB(), &ws, httpClient, force, syncedAt)
		default:
			log.Printf("ticket-sync: platform %q not supported in Go — skipping workspace %q", ws.PMPlatform, ws.Name)
		}
	}
}

// PushCachedTickets reads from the local SQLite tables (no API call) and
// pushes to Python. Called on daemon startup to immediately populate the
// server cache from the last known local state.
func PushCachedTickets(database *Database) {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		return
	}

	httpClient := trigger.NewHTTPTriggerClient()
	syncedAt := time.Now().UTC().Format(time.RFC3339)

	for _, ws := range wsCfg.GetEnabledWorkspaces() {
		switch strings.ToLower(ws.PMPlatform) {
		case "github":
			tickets := readGitHubCached(database.DB(), ws.PMProject)
			if len(tickets) > 0 {
				pushToServer(httpClient, "github", ws.Name, false, syncedAt, tickets)
			}
		case "azure":
			tickets := readAzureCached(database.DB())
			if len(tickets) > 0 {
				pushToServer(httpClient, "azure", ws.Name, false, syncedAt, tickets)
			}
		case "gitlab":
			tickets := readGitLabCached(database.DB(), ws.PMProject)
			if len(tickets) > 0 {
				pushToServer(httpClient, "gitlab", ws.Name, false, syncedAt, tickets)
			}
		}
	}
}

// ── Per-platform sync + push ──────────────────────────────────────────────────

func syncGitHub(db *sql.DB, ws *config.WorkspaceConfig, client *trigger.HTTPTriggerClient, force bool, syncedAt string) {
	c, err := pm.NewGitHubClient(ws)
	if err != nil {
		log.Printf("ticket-sync github [%s]: client error: %v", ws.Name, err)
		return
	}
	username := ws.PMUsername
	if err := c.Sync(db, username); err != nil {
		log.Printf("ticket-sync github [%s]: sync error: %v", ws.Name, err)
		return
	}
	tickets := readGitHubCached(db, ws.PMProject)
	if err := pushToServer(client, "github", ws.Name, force, syncedAt, tickets); err == nil {
		log.Printf("ticket-sync github [%s]: %d tickets pushed to server", ws.Name, len(tickets))
	}
}

func syncAzure(db *sql.DB, ws *config.WorkspaceConfig, client *trigger.HTTPTriggerClient, force bool, syncedAt string) {
	c, err := pm.NewAzureClient(ws)
	if err != nil {
		log.Printf("ticket-sync azure [%s]: client error: %v", ws.Name, err)
		return
	}
	if err := c.Sync(db); err != nil {
		log.Printf("ticket-sync azure [%s]: sync error: %v", ws.Name, err)
		return
	}
	tickets := readAzureCached(db)
	if err := pushToServer(client, "azure", ws.Name, force, syncedAt, tickets); err == nil {
		log.Printf("ticket-sync azure [%s]: %d tickets pushed to server", ws.Name, len(tickets))
	}
}

func syncGitLab(db *sql.DB, ws *config.WorkspaceConfig, client *trigger.HTTPTriggerClient, force bool, syncedAt string) {
	c, err := pm.NewGitLabClient(ws)
	if err != nil {
		log.Printf("ticket-sync gitlab [%s]: client error: %v", ws.Name, err)
		return
	}
	username := ws.PMUsername
	if err := c.Sync(db, username); err != nil {
		log.Printf("ticket-sync gitlab [%s]: sync error: %v", ws.Name, err)
		return
	}
	tickets := readGitLabCached(db, ws.PMProject)
	if err := pushToServer(client, "gitlab", ws.Name, force, syncedAt, tickets); err == nil {
		log.Printf("ticket-sync gitlab [%s]: %d tickets pushed to server", ws.Name, len(tickets))
	}
}

// ── Read from local SQLite tables ─────────────────────────────────────────────

func readGitHubCached(db *sql.DB, repo string) []trigger.SlimTicket {
	issues, err := githubconn.ListCached(db)
	if err != nil {
		log.Printf("ticket-sync: read github_issues: %v", err)
		return nil
	}
	tickets := make([]trigger.SlimTicket, 0, len(issues))
	for _, iss := range issues {
		r := repo
		if r == "" {
			r = iss.Repo
		}
		tickets = append(tickets, trigger.SlimTicket{
			ID:          fmt.Sprintf("github:%s#%d", r, iss.Number),
			ExternalID:  fmt.Sprintf("%d", iss.Number),
			Source:      "github",
			Repo:        r,
			Title:       iss.Title,
			Description: truncate(iss.Body),
			Status:      iss.State,
			URL:         iss.URL,
		})
	}
	return tickets
}

func readAzureCached(db *sql.DB) []trigger.SlimTicket {
	items, err := azureconn.ListCached(db)
	if err != nil {
		log.Printf("ticket-sync: read azure_workitems: %v", err)
		return nil
	}
	tickets := make([]trigger.SlimTicket, 0, len(items))
	for _, item := range items {
		tickets = append(tickets, trigger.SlimTicket{
			ID:         fmt.Sprintf("azure:%d", item.ID),
			ExternalID: fmt.Sprintf("%d", item.ID),
			Source:     "azure",
			Title:      item.Title(),
			Status:     item.State(),
			Assignee:   item.AssignedTo(),
			URL:        item.WebURL,
		})
	}
	return tickets
}

func readGitLabCached(db *sql.DB, repo string) []trigger.SlimTicket {
	issues, err := gitlabconn.ListCached(db)
	if err != nil {
		log.Printf("ticket-sync: read gitlab_issues: %v", err)
		return nil
	}
	tickets := make([]trigger.SlimTicket, 0, len(issues))
	for _, iss := range issues {
		r := repo
		if r == "" {
			r = iss.Repo
		}
		tickets = append(tickets, trigger.SlimTicket{
			ID:          fmt.Sprintf("gitlab:%s#%d", r, iss.IID),
			ExternalID:  fmt.Sprintf("%d", iss.IID),
			Source:      "gitlab",
			Repo:        r,
			Title:       iss.Title,
			Description: truncate(iss.Body),
			Status:      iss.State,
			URL:         iss.URL,
		})
	}
	return tickets
}

// ── Push to Python ─────────────────────────────────────────────────────────────

// pushToServer sends the slim ticket payload to the Python server.
// The error is always logged; it is also returned so CLI callers can surface
// a human-friendly warning to the terminal.  A push failure never affects the
// local SQLite cache — local data is always written first.
func pushToServer(client *trigger.HTTPTriggerClient, source, workspace string, force bool, syncedAt string, tickets []trigger.SlimTicket) error {
	err := client.SendTicketSync(trigger.TicketSyncPayload{
		Source:    source,
		Workspace: workspace,
		Force:     force,
		SyncedAt:  syncedAt,
		Tickets:   tickets,
	})
	if err != nil {
		log.Printf("ticket-sync: push to server failed (%v) — local cache still updated", err)
	}
	return err
}
