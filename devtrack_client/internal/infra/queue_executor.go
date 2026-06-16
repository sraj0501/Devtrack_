package infra

import (
	"context"
	"fmt"
	"log"
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
type QueueExecutor struct {
	db            *db.Database
	triggerClient *trigger.HTTPTriggerClient
	pollInterval  time.Duration
	stopCh        chan struct{}
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

		// 3. Skip actions still inside their approval window.
		if !expiresAt.Before(now) {
			continue
		}

		// 4. Dispatch the action.
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

		// 5. Mirror the result in the local SQLite row.
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
