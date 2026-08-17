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
//  3. If the remote row has an exact local mirror, keep that mirror in sync.
//  4. Never mutate an unrelated local row that happens to share the server row's ID.
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
	triggerClient queueTriggerClient
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
	seenIDs map[string]struct{}
}

type queueTriggerClient interface {
	GetQueuePending() (*trigger.QueuePendingResponse, error)
	ExecuteQueueAction(int64) (*trigger.QueueExecuteResponse, error)
	ExecuteStagedQueueAction(db.PendingAction) (*trigger.QueueExecuteResponse, error)
	SendClientEvents(trigger.ClientEventSyncPayload) (*trigger.ClientEventSyncResponse, error)
	PostDialecticInfer(db.PendingAction) ([]trigger.InferenceResult, error)
}

// NewQueueExecutor creates a QueueExecutor that polls at the interval configured
// by QUEUE_POLL_INTERVAL_SECS. Both database and triggerClient must be non-nil.
func NewQueueExecutor(database *db.Database, client queueTriggerClient) *QueueExecutor {
	interval := time.Duration(config.GetQueuePollIntervalSecs()) * time.Second
	return &QueueExecutor{
		db:            database,
		triggerClient: client,
		pollInterval:  interval,
		stopCh:        make(chan struct{}),
		seenIDs:       make(map[string]struct{}),
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
	now := time.Now()
	q.tickLocalPendingActions(now)
	q.tickServerEventSync(now)

	// 1. Fetch pending actions from the Python server.
	resp, err := q.triggerClient.GetQueuePending()
	if err != nil {
		log.Printf("queue executor: GET /queue/pending failed: %v", err)
		return
	}

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
			remoteAction := pendingActionFromServer(action)
			if action.ActionType == "eod_report" {
				q.maybeEODReportAction("server", remoteAction)
			} else {
				q.maybeNotifyAction("server", remoteAction)
			}
			continue
		}

		// 3b. Expired window — check adaptive threshold before dispatching.
		//     PostgreSQL is authoritative for server queue rows. A local SQLite row may
		//     independently use the same numeric ID, so only treat it as a mirror when
		//     its identifying content exactly matches the remote action.
		//     If the action's confidence is below the per-type adaptive threshold,
		//     defer it (leave in pending state for manual review) rather than auto-approving.
		remoteAction := pendingActionFromServer(action)
		localMirror := q.matchingLocalAction(remoteAction)
		if q.db != nil {
			threshold, threshErr := q.db.GetOrCreateThreshold(remoteAction.ActionType, remoteAction.Workspace)
			if threshErr == nil && remoteAction.Confidence < threshold.Threshold {
				log.Printf("queue: deferring action %d (type=%s conf=%.2f below threshold=%.2f)",
					action.ID, remoteAction.ActionType, remoteAction.Confidence, threshold.Threshold)
				continue
			}
			if threshErr != nil {
				// Fail open — proceed with execution when threshold lookup fails.
				log.Printf("queue: threshold lookup failed for action %d: %v — proceeding with execution", action.ID, threshErr)
			}
		}

		// 3c. Dispatch the action.
		log.Printf("queue: auto-approving action %d (type=%s target=%s)", action.ID, action.ActionType, action.Target)
		execResp, execErr := q.triggerClient.ExecuteQueueAction(action.ID)
		if execErr != nil {
			// HTTP-level failure: Python server unreachable or returned non-2xx.
			log.Printf("queue executor: POST /queue/execute failed for action %d: %v", action.ID, execErr)
			if localMirror != nil {
				if dbErr := q.db.UpdatePendingActionError(action.ID, execErr.Error()); dbErr != nil {
					log.Printf("queue executor: failed to record error for action %d: %v", action.ID, dbErr)
				}
			}
			continue
		}

		// 4. Mirror the result only when this server row has an exact local counterpart.
		if execResp.Status == "posted" {
			log.Printf("queue: auto-approved action %d (type=%s target=%s)", action.ID, action.ActionType, action.Target)
			if q.db != nil {
				if localMirror != nil {
					if dbErr := q.db.UpdatePendingActionStatus(action.ID, "posted", "auto"); dbErr != nil {
						log.Printf("queue executor: failed to mark action %d as posted: %v", action.ID, dbErr)
					}
				}
				// Adaptive threshold signal — record auto-approval for per-type threshold learning.
				if logErr := q.db.RecordApproval(remoteAction.ActionType, remoteAction.Workspace); logErr != nil {
					log.Printf("[threshold] RecordApproval (auto): %v", logErr)
				}
			}

			// 5. Fire-and-forget: call /dialectic/infer and store the returned
			//    inferences in SQLite. Use the authoritative server action so an
			//    unrelated local row cannot influence the inference request.
			if q.db != nil {
				actedAt := time.Now()
				actedBy := "auto"
				remoteAction.Status = "posted"
				remoteAction.ActedAt = &actedAt
				remoteAction.ActedBy = &actedBy
				go q.fireDialecticInfer(remoteAction)
			}
		} else {
			// status == "failed" or unexpected value
			errMsg := execResp.Error
			if errMsg == "" {
				errMsg = "execution failed (no error message from server)"
			}
			log.Printf("queue executor: action %d failed: %s", action.ID, errMsg)
			if localMirror != nil {
				if dbErr := q.db.UpdatePendingActionError(action.ID, errMsg); dbErr != nil {
					log.Printf("queue executor: failed to record failure for action %d: %v", action.ID, dbErr)
				}
			}
		}
	}
}

// tickLocalPendingActions dispatches Go-originated actions from the local
// trust queue. In PostgreSQL mode those rows no longer share a database or ID
// sequence with the Python queue, so the complete already-staged action is
// sent over the authenticated boundary after approval/expiry.
func (q *QueueExecutor) tickLocalPendingActions(now time.Time) {
	if q.db == nil || q.triggerClient == nil {
		return
	}
	pending, err := q.db.ListPendingActions("pending")
	if err != nil {
		log.Printf("queue executor: list local pending actions: %v", err)
		return
	}
	approved, err := q.db.ListPendingActions("approved")
	if err != nil {
		log.Printf("queue executor: list local approved actions: %v", err)
		return
	}
	actions := append(pending, approved...)
	for _, action := range actions {
		if action.ActionType == db.ServerEventSyncActionType {
			continue
		}
		if action.Status == "pending" && !action.ExpiresAt.Before(now) {
			if action.ActionType == "eod_report" {
				q.maybeEODReportAction("local", action)
			} else {
				q.maybeNotifyAction("local", action)
			}
			continue
		}
		if action.Status == "pending" {
			threshold, thresholdErr := q.db.GetOrCreateThreshold(action.ActionType, action.Workspace)
			if thresholdErr == nil && action.Confidence < threshold.Threshold {
				continue
			}
			if thresholdErr != nil {
				log.Printf("queue: local threshold lookup failed for action %d: %v — proceeding with execution", action.ID, thresholdErr)
			}
		}

		response, execErr := q.triggerClient.ExecuteStagedQueueAction(action)
		if execErr != nil {
			log.Printf("queue executor: local action %d retained for retry: %v", action.ID, execErr)
			if dbErr := q.db.RecordPendingActionAttemptError(action.ID, execErr.Error()); dbErr != nil {
				log.Printf("queue executor: record local action %d retry error: %v", action.ID, dbErr)
			}
			continue
		}
		if response.Status == "posted" {
			if dbErr := q.db.UpdatePendingActionStatus(action.ID, "posted", "auto"); dbErr != nil {
				log.Printf("queue executor: mark local action %d posted: %v", action.ID, dbErr)
				continue
			}
			if logErr := q.db.RecordApproval(action.ActionType, action.Workspace); logErr != nil {
				log.Printf("[threshold] RecordApproval (local auto): %v", logErr)
			}
			actedAt := time.Now()
			actedBy := "auto"
			action.Status = "posted"
			action.ActedAt = &actedAt
			action.ActedBy = &actedBy
			go q.fireDialecticInfer(action)
			continue
		}
		errMessage := response.Error
		if errMessage == "" {
			errMessage = "execution failed (no error message from server)"
		}
		if dbErr := q.db.UpdatePendingActionError(action.ID, errMessage); dbErr != nil {
			log.Printf("queue executor: mark local action %d failed: %v", action.ID, dbErr)
		}
	}
}

// matchingLocalAction returns a local row only when it is a genuine mirror of
// the PostgreSQL action. Numeric IDs are database-local and cannot establish
// identity across the server and client stores by themselves.
func (q *QueueExecutor) matchingLocalAction(remote db.PendingAction) *db.PendingAction {
	if q.db == nil {
		return nil
	}
	local, err := q.db.GetPendingAction(remote.ID)
	if err != nil {
		log.Printf("queue executor: cannot inspect local mirror for server action %d: %v", remote.ID, err)
		return nil
	}
	if local == nil ||
		local.ActionType != remote.ActionType ||
		local.Target != remote.Target ||
		local.Platform != remote.Platform ||
		local.Workspace != remote.Workspace ||
		local.Payload != remote.Payload ||
		local.Confidence != remote.Confidence {
		return nil
	}
	return local
}

// tickServerEventSync stages and dispatches the opt-in local event outbox.
// It runs before the server queue poll so an unreachable server never drops
// the local backlog or blocks the rest of the daemon.
func (q *QueueExecutor) tickServerEventSync(now time.Time) {
	if q.db == nil || !config.GetServerEventSyncEnabled() {
		return
	}
	action, err := q.db.StageServerEventSync(config.GetServerEventSyncBatchSize(), now)
	if err != nil {
		log.Printf("server event sync: could not stage batch: %v", err)
		return
	}
	if action == nil {
		return
	}
	if action.Status != "approved" && action.ExpiresAt.After(now) {
		return
	}

	payload, err := db.DecodeServerEventSyncAction(*action)
	if err != nil {
		log.Printf("server event sync: invalid staged action %d: %v", action.ID, err)
		_ = q.db.UpdatePendingActionError(action.ID, err.Error())
		return
	}
	request := trigger.ClientEventSyncPayload{
		ClientID: payload.ClientID,
		Events:   make([]trigger.ClientEvent, 0, len(payload.Events)),
	}
	for _, event := range payload.Events {
		request.Events = append(request.Events, trigger.ClientEvent{
			EventID:     event.EventID,
			TableName:   event.TableName,
			SourceRowID: event.SourceRowID,
			Revision:    event.Revision,
			Payload:     event.Payload,
			UpdatedAt:   event.UpdatedAt,
		})
	}
	if _, err := q.triggerClient.SendClientEvents(request); err != nil {
		log.Printf("server event sync: batch %d retained for replay: %v", action.ID, err)
		_ = q.db.RecordServerEventSyncFailure(db.ServerEventKeys(payload), err.Error())
		return
	}
	if err := q.db.MarkServerEventsSynced(db.ServerEventKeys(payload)); err != nil {
		log.Printf("server event sync: server accepted batch %d but local acknowledgement failed: %v", action.ID, err)
		return
	}
	if err := q.db.UpdatePendingActionStatus(action.ID, "posted", "auto"); err != nil {
		log.Printf("server event sync: could not mark action %d posted: %v", action.ID, err)
		return
	}
	log.Printf("server event sync: accepted %d event(s) from action %d", len(payload.Events), action.ID)
}

// maybeNotifyAction fires NotifyFn once per new low-confidence action. The
// full action comes from the server response in PostgreSQL mode.
func (q *QueueExecutor) maybeNotifyAction(origin string, action db.PendingAction) {
	if q.NotifyFn == nil {
		return
	}
	q.seenMu.Lock()
	seenKey := fmt.Sprintf("%s:%d", origin, action.ID)
	_, alreadySeen := q.seenIDs[seenKey]
	if alreadySeen {
		q.seenMu.Unlock()
		return
	}
	q.seenIDs[seenKey] = struct{}{}
	q.seenMu.Unlock()

	// Only notify for low-confidence actions (< 0.90). High-confidence ones
	// auto-approve within 2 minutes and are not worth interrupting the user for.
	if action.Confidence >= 0.90 {
		return
	}

	go q.NotifyFn(action)
}

// maybeEODReportAction delivers an EOD report from the full server queue row.
func (q *QueueExecutor) maybeEODReportAction(origin string, action db.PendingAction) {
	if q.EODReportFn == nil || !config.GetEODTelegramEnabled() {
		return
	}

	q.seenMu.Lock()
	seenKey := fmt.Sprintf("%s:%d", origin, action.ID)
	_, alreadySeen := q.seenIDs[seenKey]
	if alreadySeen {
		q.seenMu.Unlock()
		return
	}
	q.seenIDs[seenKey] = struct{}{}
	q.seenMu.Unlock()

	// Extract narrative and date from the JSON payload.
	var payload struct {
		Narrative string `json:"narrative"`
		Date      string `json:"date"`
	}
	if jsonErr := json.Unmarshal([]byte(action.Payload), &payload); jsonErr != nil {
		log.Printf("queue executor: eod_report action %d: cannot parse payload: %v", action.ID, jsonErr)
		return
	}
	if payload.Narrative == "" {
		log.Printf("queue executor: eod_report action %d: payload has no narrative — skipping Telegram delivery", action.ID)
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

func pendingActionFromServer(action trigger.QueuePendingAction) db.PendingAction {
	expiresAt, _ := parseISO8601(action.ExpiresAt)
	createdAt, _ := parseISO8601(action.CreatedAt)
	result := db.PendingAction{
		ID: action.ID, ActionType: action.ActionType, Target: action.Target,
		Platform: action.Platform, Workspace: action.Workspace, Payload: action.Payload,
		Confidence: action.Confidence, Status: action.Status,
		ExpiresAt: expiresAt, CreatedAt: createdAt,
		ActedBy: action.ActedBy, Error: action.Error,
	}
	if action.ActedAt != nil {
		if actedAt, err := parseISO8601(*action.ActedAt); err == nil {
			result.ActedAt = &actedAt
		}
	}
	return result
}

// fireDialecticInfer calls POST /dialectic/infer for a completed queue action
// and stores each returned inference in the local SQLite inferences table.
// This method is always called as a goroutine — it logs errors and never panics.
func (q *QueueExecutor) fireDialecticInfer(action db.PendingAction) {
	inferences, err := q.triggerClient.PostDialecticInfer(action)
	if err != nil {
		log.Printf("dialectic: infer call failed for action %d: %v", action.ID, err)
		return
	}
	for _, inf := range inferences {
		_, storeErr := q.db.InsertInference(db.Inference{
			ContextType: inf.ContextType,
			Subject:     inf.Subject,
			Inference:   inf.InferenceText,
			Evidence:    fmt.Sprintf(`[%d]`, action.ID),
			Confidence:  inf.Confidence,
			Source:      "hermes3",
		})
		if storeErr != nil {
			log.Printf("dialectic: store inference failed for action %d: %v", action.ID, storeErr)
		}
	}
	if len(inferences) > 0 {
		log.Printf("dialectic: stored %d inference(s) for action %d", len(inferences), action.ID)
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
