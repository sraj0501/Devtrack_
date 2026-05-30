package pm

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// ListOpenTicketsCached returns the workspace's open tickets, preferring the
// live API and writing results through to the ticket_cache table. When the API
// is unreachable it falls back to the local cache. fromCache reports whether
// the returned tickets came from the offline cache.
func ListOpenTicketsCached(ws *config.WorkspaceConfig, database *db.Database) (tickets []Ticket, fromCache bool, err error) {
	live, liveErr := ListOpenTickets(ws)
	if liveErr == nil {
		if database != nil {
			cacheTickets(database, ws, live)
		}
		return live, false, nil
	}

	// API failed — try the offline cache before giving up.
	if database != nil {
		if cached, cErr := cachedTickets(database, ws); cErr == nil && len(cached) > 0 {
			return cached, true, nil
		}
	}
	return nil, false, liveErr
}

// assigneeKey is the stable key tickets are cached/retrieved under for a
// workspace: the configured pm_assignee, else a platform username, else "self".
func assigneeKey(ws *config.WorkspaceConfig) string {
	if ws != nil && strings.TrimSpace(ws.PMAssignee) != "" {
		return strings.ToLower(strings.TrimSpace(ws.PMAssignee))
	}
	if ws != nil {
		switch strings.ToLower(ws.PMPlatform) {
		case "github":
			if u := os.Getenv("GITHUB_USERNAME"); u != "" {
				return strings.ToLower(u)
			}
		case "gitlab":
			if u := os.Getenv("GITLAB_USERNAME"); u != "" {
				return strings.ToLower(u)
			}
		}
	}
	return "self"
}

// cacheTickets writes tickets through to the offline ticket_cache (best-effort).
func cacheTickets(database *db.Database, ws *config.WorkspaceConfig, tickets []Ticket) {
	key := assigneeKey(ws)
	now := time.Now().UTC()
	for _, t := range tickets {
		_ = database.UpsertTicketCache(db.TicketCacheRecord{
			ID:          t.Platform + ":" + strconv.Itoa(t.Number),
			Source:      t.Platform,
			ExternalID:  strconv.Itoa(t.Number),
			Repo:        t.Repo,
			Title:       t.Title,
			Description: t.Body,
			Status:      t.State,
			Assignee:    key,
			URL:         t.URL,
			SyncedAt:    now,
		})
	}
}

// cachedTickets reads the workspace platform's tickets back from ticket_cache.
func cachedTickets(database *db.Database, ws *config.WorkspaceConfig) ([]Ticket, error) {
	recs, err := database.GetTicketsByAssignee(assigneeKey(ws))
	if err != nil {
		return nil, err
	}
	platform := ""
	if ws != nil {
		platform = strings.ToLower(ws.PMPlatform)
	}
	out := make([]Ticket, 0, len(recs))
	for _, r := range recs {
		if strings.ToLower(r.Source) != platform {
			continue
		}
		num, _ := strconv.Atoi(r.ExternalID)
		out = append(out, Ticket{
			Platform: r.Source,
			Number:   num,
			ID:       displayID(r.Source, num),
			Title:    r.Title,
			Body:     r.Description,
			State:    r.Status,
			URL:      r.URL,
			Repo:     r.Repo,
		})
	}
	return out, nil
}

// displayID rebuilds the platform-specific display id from a number.
func displayID(platform string, number int) string {
	if strings.EqualFold(platform, "azure") {
		return "AB#" + strconv.Itoa(number)
	}
	return "#" + strconv.Itoa(number)
}
