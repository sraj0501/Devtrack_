package pm

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// ActionComment is the pm_update_queue action for a queued ticket comment.
const ActionComment = "comment"

// QueuedComment is the JSON payload stored in pm_update_queue for a deferred
// comment, so the daemon flusher can replay it verbatim when back online.
type QueuedComment struct {
	Ticket Ticket `json:"ticket"`
	Body   string `json:"body"`
}

// EnqueueComment stores a comment in the offline outbox (pm_update_queue).
func EnqueueComment(database *db.Database, t Ticket, body, commitHash string) error {
	payload, err := json.Marshal(QueuedComment{Ticket: t, Body: body})
	if err != nil {
		return err
	}
	_, err = database.InsertPMUpdateQueue(db.PMUpdateQueueRecord{
		TicketID:   t.ID,
		Action:     ActionComment,
		Payload:    string(payload),
		CommitHash: commitHash,
	})
	return err
}

// StagePendingComment stages a direct-ticket comment in pending_actions — the
// trust primitive (Non-Negotiable #2) — instead of posting straight to the PM
// API. The queue executor posts it via the server's direct-by-ID comment path
// after the confidence timeout; until then it is visible and editable in
// `devtrack queue list`.
func StagePendingComment(database *db.Database, ws *config.WorkspaceConfig, t Ticket, body string) (int64, error) {
	pmProject := ""
	wsName := ""
	if ws != nil {
		pmProject = ws.PMProject
		wsName = ws.Name
	}
	payload, err := json.Marshal(map[string]any{
		"comment":       body,
		"description":   body,
		"ticket_id":     t.ID,
		"direct_ticket": true,
		"pm_project":    pmProject,
	})
	if err != nil {
		return 0, err
	}
	// Confidence 1.0: the developer explicitly confirmed this post — shortest
	// auto-approve window, still queue-visible.
	return database.InsertPendingAction(db.PendingAction{
		ActionType: "post_comment",
		Target:     t.ID,
		Platform:   strings.ToLower(t.Platform),
		Workspace:  wsName,
		Confidence: 1.0,
		Payload:    string(payload),
		Status:     "pending",
		ExpiresAt:  time.Now().Add(db.ConfidenceTimeout(1.0, false)),
	})
}

// FlushQueue drains all pending 'comment' PM updates from the offline outbox.
// When the Python server is reachable, entries are staged into pending_actions
// (queue-first, TASK-127) so retried posts stay visible in `devtrack queue
// list`; without a server (lightweight/offline) they post directly as before.
// Returns how many were sent/staged and how many failed (and remain pending).
func FlushQueue(database *db.Database) (sent, failed int) {
	pending, err := database.GetPendingPMUpdates()
	if err != nil || len(pending) == 0 {
		return 0, 0
	}

	// Load all workspaces once so we can find the right config per ticket.
	wsCfg, _ := config.LoadWorkspacesConfig()
	serverUp := trigger.NewHTTPTriggerClient().Ping()

	for _, rec := range pending {
		if rec.Action != ActionComment {
			continue
		}
		var qc QueuedComment
		if err := json.Unmarshal([]byte(rec.Payload), &qc); err != nil {
			_ = database.MarkPMUpdateFailed(rec.ID, "bad payload: "+err.Error())
			failed++
			continue
		}
		// Find the best workspace for this ticket's platform.
		ws := workspaceForPlatform(wsCfg, qc.Ticket.Platform)

		if serverUp {
			// Queue-first: stage and let the executor post it.
			if _, err := StagePendingComment(database, ws, qc.Ticket, qc.Body); err != nil {
				_ = database.MarkPMUpdateFailed(rec.ID, err.Error())
				failed++
				continue
			}
			_ = database.MarkPMUpdateSent(rec.ID)
			sent++
			continue
		}

		if err := AddComment(ws, qc.Ticket, qc.Body); err != nil {
			_ = database.MarkPMUpdateFailed(rec.ID, err.Error())
			failed++
			continue
		}
		_ = database.MarkPMUpdateSent(rec.ID)
		sent++
	}
	return sent, failed
}

// workspaceForPlatform returns the first enabled workspace matching platform,
// or nil if none found (AddComment will use default API URLs for GitHub/GitLab).
func workspaceForPlatform(wsCfg *config.WorkspacesConfig, platform string) *config.WorkspaceConfig {
	if wsCfg == nil {
		return nil
	}
	for i := range wsCfg.Workspaces {
		ws := &wsCfg.Workspaces[i]
		if ws.Enabled && strings.EqualFold(ws.PMPlatform, platform) {
			return ws
		}
	}
	return nil
}
