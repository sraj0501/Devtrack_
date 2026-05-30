package pm

import (
	"encoding/json"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
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

// FlushQueue attempts to send all pending 'comment' PM updates from the outbox.
// Returns how many were sent and how many failed (and remain pending).
func FlushQueue(database *db.Database) (sent, failed int) {
	pending, err := database.GetPendingPMUpdates()
	if err != nil {
		return 0, 0
	}

	// Load all workspaces once so we can find the right config per ticket.
	wsCfg, _ := config.LoadWorkspacesConfig()

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
