package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// QueueExecutor polls the Python server for pending actions and auto-approves
// those whose timeout has expired. It is a self-contained goroutine started by
// IntegratedMonitor alongside the git monitor and scheduler.
//
// On each tick:
//  1. Call GET /queue/pending — get all actions with status='pending'.
//  2. For each action whose expires_at is in the past: call POST /queue/execute.
//  3. If execution succeeds: mark the local SQLite row status='posted' (actedBy="auto").
//  4. If execution fails: mark the local SQLite row status='failed' with the error.
//
// HTTP errors are logged and do not crash the executor — next tick will retry.
// Actions whose expires_at is still in the future are skipped (still in review window).
// Actions that were manually approved/rejected (status no longer 'pending') are
// skipped because GET /queue/pending only returns rows with status='pending'.
//
// When NotifyFn is set, the executor calls it for each new pending action whose
// confidence < 0.90 (i.e. not imminently auto-approved). This enables Telegram
// or other notification channels to push proactive messages to the user.
//
// When EODReportFn is set and EOD_TELEGRAM_ENABLED=true, the executor calls it
// once for each new pending eod_report action (TASK-078 channel parity). The
// callback receives the narrative text, date string, and action ID so the
// Telegram bot can send the report with Approve/Reject inline keyboard buttons.
type QueueExecutor struct {
	db            *db.Database
	triggerClient *trigger.HTTPTriggerClient
	pollInterval  time.Duration
	stopCh        chan struct{}

	// NotifyFn is an optional callback invoked once for each new pending action
	// with confidence < 0.90. The executor tracks which action IDs it has already
	// notified so the callback fires only once per action, not on every poll tick.
	// Safe to set before calling Start(). Nil means no notification.
	NotifyFn func(action db.PendingAction)

	// EODReportFn is an optional callback invoked once for each new pending action
	// with action_type == "eod_report" when EOD_TELEGRAM_ENABLED=true.
	// Signature: func(narrative, date string, actionID int64) error
	// Called before the standard auto-approve check so the user receives the report
	// via Telegram before the approval window expires. Nil means no Telegram delivery.
	EODReportFn func(narrative, date string, actionID int64) error

	// seenIDs is the set of action IDs we have already sent a notification for.
	seenMu  sync.Mutex
	seenIDs map[int64]struct{}
}

// NewQueueExecutor creates a QueueExecutor that polls at the interval configured
// by QUEUE_POLL_INTERVAL_SECS. Both database and triggerClient must be non-nil.
func NewQueueExecutor(database *db.Database, client *trigger.HTTPTriggerClient) *QueueExecutor {
	interval := time.Duration(config.GetQueuePollIntervalSecs()) * time.Second
	return &QueueExecutor{
		db:            database,
		triggerClient: client,
		pollInterval:  interval,
		stopCh:        make(chan struct{}),
		seenIDs:       make(map[int64]struct{}),
	}
}

// Start runs the executor's poll loop until ctx is cancelled or Stop() is called.
// Intended to be run as a goroutine: go executor.Start(ctx).
func (q *QueueExecutor) Start(ctx context.Context) {
	log.Printf("queue executor: started (poll interval: %s)", q.pollInterval)
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("queue executor: context cancelled — stopping")
			return
		case <-q.stopCh:
			log.Println("queue executor: Stop() called — stopping")
			return
		case <-ticker.C:
			q.tick()
		}
	}
}

// Stop signals the executor to exit. Safe to call multiple times.
func (q *QueueExecutor) Stop() {
	select {
	case <-q.stopCh:
		// already closed — no-op
	default:
		close(q.stopCh)
	}
}

// tick is the single poll iteration. Called by the ticker in Start().
// Never panics; all errors are logged and execution continues.
func (q *QueueExecutor) tick() {
	// 1. Fetch pending actions from the Python server.
	resp, err := q.triggerClient.GetQueuePending()
	if err != nil {
		log.Printf("queue executor: GET /queue/pending failed: %v", err)
		return
	}

	now := time.Now()
	for _, action := range resp.Actions {
		if action.Status != "pending" {
			// Defensive: Python should only return 'pending' rows, but skip any others.
			continue
		}

		// 2. Parse expires_at. Try ISO 8601 without timezone first (Python stores in UTC).
		expiresAt, parseErr := parseISO8601(action.ExpiresAt)
		if parseErr != nil {
			log.Printf("queue executor: cannot parse expires_at %q for action %d: %v", action.ExpiresAt, action.ID, parseErr)
			continue
		}

		// 3a. If inside approval window: send Telegram notifications for new actions.
		//     For eod_report actions, deliver the report narrative (TASK-078 channel parity).
		//     For other low-confidence actions, send the standard pending-action notification.
		if !expiresAt.Before(now) {
			if action.ActionType == "eod_report" {
				q.maybeEODReport(action.ID)
			} else {
				q.maybeNotify(action.ID)
			}
			continue
		}

		// 3b. Expired window — dispatch the action.
		log.Printf("queue: auto-approving action %d (type=%s target=%s)", action.ID, action.ActionType, action.Target)
		execResp, execErr := q.triggerClient.ExecuteQueueAction(action.ID)
		if execErr != nil {
			// HTTP-level failure: Python server unreachable or returned non-2xx.
			log.Printf("queue executor: POST /queue/execute failed for action %d: %v", action.ID, execErr)
			if q.db != nil {
				if dbErr := q.db.UpdatePendingActionError(action.ID, execErr.Error()); dbErr != nil {
					log.Printf("queue executor: failed to record error for action %d: %v", action.ID, dbErr)
				}
			}
			continue
		}

		// 4. Mirror the result in the local SQLite row.
		if execResp.Status == "posted" {
			log.Printf("queue: auto-approved action %d (type=%s target=%s)", action.ID, action.ActionType, action.Target)
			if q.db != nil {
				if dbErr := q.db.UpdatePendingActionStatus(action.ID, "posted", "auto"); dbErr != nil {
					log.Printf("queue executor: failed to mark action %d as posted: %v", action.ID, dbErr)
				}
			}
		} else {
			// status == "failed" or unexpected value
			errMsg := execResp.Error
			if errMsg == "" {
				errMsg = "execution failed (no error message from server)"
			}
			log.Printf("queue executor: action %d failed: %s", action.ID, errMsg)
			if q.db != nil {
				if dbErr := q.db.UpdatePendingActionError(action.ID, errMsg); dbErr != nil {
					log.Printf("queue executor: failed to record failure for action %d: %v", action.ID, dbErr)
				}
			}
		}
	}
}

// maybeNotify fires NotifyFn once per new low-confidence pending action.
// It looks up the full PendingAction from the local DB so the notification has
// all fields (payload, confidence, etc.). If the DB lookup fails or NotifyFn is
// nil, the method is a no-op.
func (q *QueueExecutor) maybeNotify(id int64) {
	if q.NotifyFn == nil {
		return
	}

	q.seenMu.Lock()
	_, alreadySeen := q.seenIDs[id]
	if alreadySeen {
		q.seenMu.Unlock()
		return
	}
	q.seenIDs[id] = struct{}{}
	q.seenMu.Unlock()

	// Look up full action from local SQLite for the notification payload.
	if q.db == nil {
		return
	}
	action, err := q.db.GetPendingAction(id)
	if err != nil || action == nil {
		// If not in local DB yet (Python gateway writes first), skip this tick;
		// it will be retried next poll when the row propagates.
		q.seenMu.Lock()
		delete(q.seenIDs, id)
		q.seenMu.Unlock()
		return
	}

	// Only notify for low-confidence actions (< 0.90). High-confidence ones
	// auto-approve within 2 minutes and are not worth interrupting the user for.
	if action.Confidence >= 0.90 {
		return
	}

	go q.NotifyFn(*action)
}

// maybeEODReport fires EODReportFn once for a new pending eod_report action when
// EOD_TELEGRAM_ENABLED=true. It looks up the full PendingAction from local SQLite,
// extracts the narrative and date from the JSON payload, and calls EODReportFn.
// If EODReportFn is nil, EOD_TELEGRAM_ENABLED is false, or the DB lookup fails,
// the method is a no-op — Non-Negotiable #8 (never block on failure).
func (q *QueueExecutor) maybeEODReport(id int64) {
	if q.EODReportFn == nil || !config.GetEODTelegramEnabled() {
		return
	}

	q.seenMu.Lock()
	_, alreadySeen := q.seenIDs[id]
	if alreadySeen {
		q.seenMu.Unlock()
		return
	}
	q.seenIDs[id] = struct{}{}
	q.seenMu.Unlock()

	if q.db == nil {
		return
	}
	action, err := q.db.GetPendingAction(id)
	if err != nil || action == nil {
		// Row not yet propagated to local SQLite — retry next poll.
		q.seenMu.Lock()
		delete(q.seenIDs, id)
		q.seenMu.Unlock()
		return
	}

	// Extract narrative and date from the JSON payload.
	var payload struct {
		Narrative string `json:"narrative"`
		Date      string `json:"date"`
	}
	if jsonErr := json.Unmarshal([]byte(action.Payload), &payload); jsonErr != nil {
		log.Printf("queue executor: eod_report action %d: cannot parse payload: %v", id, jsonErr)
		return
	}
	if payload.Narrative == "" {
		log.Printf("queue executor: eod_report action %d: payload has no narrative — skipping Telegram delivery", id)
		return
	}

	go func() {
		if err := q.EODReportFn(payload.Narrative, payload.Date, action.ID); err != nil {
			log.Printf("queue executor: EOD Telegram delivery failed for action %d: %v", action.ID, err)
		} else {
			log.Printf("queue executor: EOD report delivered via Telegram for action %d (date=%s)", action.ID, payload.Date)
		}
	}()
}

// parseISO8601 parses the common ISO 8601 datetime formats that Python uses
// when serialising datetime objects to JSON.
func parseISO8601(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parseISO8601: cannot parse %q", s)
}
