package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ServerEventSyncActionType identifies the local trust-queue entry used for
// client-to-server event batches.
const ServerEventSyncActionType = "server_event_sync"

// ServerEvent is one latest-state snapshot queued for the Python server.
// EventID is stable within a client, so replaying a batch is idempotent.
type ServerEvent struct {
	EventID     string         `json:"event_id"`
	TableName   string         `json:"table_name"`
	SourceRowID string         `json:"source_row_id"`
	Revision    int            `json:"revision"`
	Payload     map[string]any `json:"payload"`
	UpdatedAt   string         `json:"updated_at"`
}

// ServerEventKey identifies the exact outbox revision acknowledged by the
// server. A newer local revision must remain pending.
type ServerEventKey struct {
	EventID  string
	Revision int
}

// ServerEventSyncPayload is staged in pending_actions before any local data
// leaves the machine. The server uses (ClientID, EventID) as its replay key.
type ServerEventSyncPayload struct {
	ClientID string        `json:"client_id"`
	Events   []ServerEvent `json:"events"`
}

func (d *Database) initServerEventSync() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS server_event_outbox (
			event_id      TEXT PRIMARY KEY,
			table_name    TEXT NOT NULL,
			source_row_id TEXT NOT NULL,
			payload       TEXT NOT NULL,
			revision      INTEGER NOT NULL DEFAULT 1,
			status        TEXT NOT NULL DEFAULT 'pending',
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error    TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
			synced_at     TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_server_event_outbox_status
			ON server_event_outbox(status, updated_at)`,
		`CREATE TRIGGER IF NOT EXISTS sync_triggers_insert
		AFTER INSERT ON triggers BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'triggers:' || NEW.id, 'triggers', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'trigger_type', NEW.trigger_type, 'timestamp', NEW.timestamp,
					'source', NEW.source, 'repo_path', NEW.repo_path,
					'commit_hash', NEW.commit_hash, 'commit_message', NEW.commit_message,
					'author', NEW.author, 'data', NEW.data, 'processed', NEW.processed,
					'ticket_id', NEW.ticket_id, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_triggers_update
		AFTER UPDATE ON triggers BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'triggers:' || NEW.id, 'triggers', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'trigger_type', NEW.trigger_type, 'timestamp', NEW.timestamp,
					'source', NEW.source, 'repo_path', NEW.repo_path,
					'commit_hash', NEW.commit_hash, 'commit_message', NEW.commit_message,
					'author', NEW.author, 'data', NEW.data, 'processed', NEW.processed,
					'ticket_id', NEW.ticket_id, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_task_updates_insert
		AFTER INSERT ON task_updates BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'task_updates:' || NEW.id, 'task_updates', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'response_id', NEW.response_id, 'timestamp', NEW.timestamp,
					'project', NEW.project, 'ticket_id', NEW.ticket_id,
					'update_text', NEW.update_text, 'status', NEW.status, 'synced', NEW.synced,
					'synced_at', NEW.synced_at, 'platform', NEW.platform,
					'error', NEW.error, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_task_updates_update
		AFTER UPDATE ON task_updates BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'task_updates:' || NEW.id, 'task_updates', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'response_id', NEW.response_id, 'timestamp', NEW.timestamp,
					'project', NEW.project, 'ticket_id', NEW.ticket_id,
					'update_text', NEW.update_text, 'status', NEW.status, 'synced', NEW.synced,
					'synced_at', NEW.synced_at, 'platform', NEW.platform,
					'error', NEW.error, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_work_sessions_insert
		AFTER INSERT ON work_sessions BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'work_sessions:' || NEW.id, 'work_sessions', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'started_at', NEW.started_at, 'ended_at', NEW.ended_at,
					'ticket_ref', NEW.ticket_ref, 'repo_path', NEW.repo_path,
					'workspace_name', NEW.workspace_name, 'description', NEW.description,
					'commits', NEW.commits, 'duration_minutes', NEW.duration_minutes,
					'adjusted_minutes', NEW.adjusted_minutes,
					'auto_stopped', NEW.auto_stopped, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_work_sessions_update
		AFTER UPDATE ON work_sessions BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'work_sessions:' || NEW.id, 'work_sessions', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'started_at', NEW.started_at, 'ended_at', NEW.ended_at,
					'ticket_ref', NEW.ticket_ref, 'repo_path', NEW.repo_path,
					'workspace_name', NEW.workspace_name, 'description', NEW.description,
					'commits', NEW.commits, 'duration_minutes', NEW.duration_minutes,
					'adjusted_minutes', NEW.adjusted_minutes,
					'auto_stopped', NEW.auto_stopped, 'created_at', NEW.created_at
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_pending_actions_insert
		AFTER INSERT ON pending_actions
	WHEN NEW.action_type <> 'server_event_sync' BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'pending_actions:' || NEW.id, 'pending_actions', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'action_type', NEW.action_type, 'target', NEW.target,
					'platform', NEW.platform, 'workspace', NEW.workspace,
					'payload', NEW.payload, 'confidence', NEW.confidence, 'status', NEW.status,
					'expires_at', NEW.expires_at, 'created_at', NEW.created_at,
					'acted_at', NEW.acted_at, 'acted_by', NEW.acted_by, 'error', NEW.error
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
		`CREATE TRIGGER IF NOT EXISTS sync_pending_actions_update
		AFTER UPDATE ON pending_actions
	WHEN NEW.action_type <> 'server_event_sync' BEGIN
			INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload)
			VALUES (
				'pending_actions:' || NEW.id, 'pending_actions', CAST(NEW.id AS TEXT),
				json_object(
					'id', NEW.id, 'action_type', NEW.action_type, 'target', NEW.target,
					'platform', NEW.platform, 'workspace', NEW.workspace,
					'payload', NEW.payload, 'confidence', NEW.confidence, 'status', NEW.status,
					'expires_at', NEW.expires_at, 'created_at', NEW.created_at,
					'acted_at', NEW.acted_at, 'acted_by', NEW.acted_by, 'error', NEW.error
				)
			)
			ON CONFLICT(event_id) DO UPDATE SET
				payload = excluded.payload, revision = server_event_outbox.revision + 1,
				status = 'pending', last_error = NULL,
				updated_at = datetime('now'), synced_at = NULL;
		END`,
	}
	for _, statement := range statements {
		if _, err := d.db.Exec(statement); err != nil {
			return fmt.Errorf("server event sync schema: %w", err)
		}
	}
	return nil
}

// GetOrCreateServerEventClientID returns a stable random identifier stored in
// the local SQLite config table. It is not a hostname and contains no user data.
func (d *Database) GetOrCreateServerEventClientID() (string, error) {
	const key = "server_event_sync_client_id"
	var clientID string
	err := d.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&clientID)
	if err == nil && clientID != "" {
		return clientID, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("read server event client id: %w", err)
	}
	clientID = uuid.NewString()
	if _, err := d.db.Exec(
		`INSERT INTO config(key, value, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		key, clientID,
	); err != nil {
		return "", fmt.Errorf("store server event client id: %w", err)
	}
	return clientID, nil
}

// ListPendingServerEvents returns the oldest unsent row snapshots first.
func (d *Database) ListPendingServerEvents(limit int) ([]ServerEvent, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("server event batch limit must be positive")
	}
	rows, err := d.db.Query(`
		SELECT event_id, table_name, source_row_id, revision, payload, updated_at
		FROM server_event_outbox
		WHERE status = 'pending'
		ORDER BY updated_at ASC, event_id ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending server events: %w", err)
	}
	defer rows.Close()

	events := make([]ServerEvent, 0)
	for rows.Next() {
		var event ServerEvent
		var payload string
		if err := rows.Scan(
			&event.EventID, &event.TableName, &event.SourceRowID, &event.Revision,
			&payload, &event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending server event: %w", err)
		}
		if err := json.Unmarshal([]byte(payload), &event.Payload); err != nil {
			return nil, fmt.Errorf("decode payload for %s: %w", event.EventID, err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// StageServerEventSync creates at most one local review action for the next
// outbox batch. A nil result means there is nothing new to stage or an action
// is already awaiting dispatch.
func (d *Database) StageServerEventSync(limit int, now time.Time) (*PendingAction, error) {
	existing, err := d.pendingServerEventSyncAction()
	if err != nil || existing != nil {
		return existing, err
	}
	events, err := d.ListPendingServerEvents(limit)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	clientID, err := d.GetOrCreateServerEventClientID()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(ServerEventSyncPayload{ClientID: clientID, Events: events})
	if err != nil {
		return nil, fmt.Errorf("encode server event sync batch: %w", err)
	}
	action := PendingAction{
		ActionType: ServerEventSyncActionType,
		Target:     "configured DevTrack server",
		Platform:   "devtrack_server",
		Workspace:  "all",
		Payload:    string(payload),
		Confidence: 1.0,
		Status:     "pending",
		ExpiresAt:  now.UTC().Add(ConfidenceTimeout(1.0, false)),
	}
	id, err := d.InsertPendingAction(action)
	if err != nil {
		return nil, err
	}
	action.ID = id
	action.CreatedAt = now.UTC()
	return &action, nil
}

func (d *Database) pendingServerEventSyncAction() (*PendingAction, error) {
	row := d.db.QueryRow(`
		SELECT id, action_type, target, platform, workspace, payload, confidence,
		       status, expires_at, created_at, acted_at, acted_by, error
		FROM pending_actions
		WHERE action_type = ? AND status IN ('pending', 'approved')
		ORDER BY created_at ASC LIMIT 1`, ServerEventSyncActionType)
	action, err := scanPendingActionRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pending server event sync action: %w", err)
	}
	return action, nil
}

// DecodeServerEventSyncAction validates and decodes a staged sync payload.
func DecodeServerEventSyncAction(action PendingAction) (ServerEventSyncPayload, error) {
	if action.ActionType != ServerEventSyncActionType {
		return ServerEventSyncPayload{}, fmt.Errorf("action %d is not a server event sync", action.ID)
	}
	var payload ServerEventSyncPayload
	if err := json.Unmarshal([]byte(action.Payload), &payload); err != nil {
		return ServerEventSyncPayload{}, fmt.Errorf("decode action %d payload: %w", action.ID, err)
	}
	if payload.ClientID == "" || len(payload.Events) == 0 {
		return ServerEventSyncPayload{}, fmt.Errorf("action %d has an empty sync payload", action.ID)
	}
	return payload, nil
}

// MarkServerEventsSynced marks exactly the acknowledged event keys as sent.
func (d *Database) MarkServerEventsSynced(events []ServerEventKey) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin mark server events synced: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, event := range events {
		if _, err := tx.Exec(`
			UPDATE server_event_outbox
			SET status = 'synced', synced_at = datetime('now'), last_error = NULL
			WHERE event_id = ? AND revision = ?`, event.EventID, event.Revision); err != nil {
			return fmt.Errorf("mark server event %s synced: %w", event.EventID, err)
		}
	}
	return tx.Commit()
}

// RecordServerEventSyncFailure retains the rows for replay and records the
// latest transport/server error without exposing it as a daemon failure.
func (d *Database) RecordServerEventSyncFailure(events []ServerEventKey, message string) error {
	for _, event := range events {
		if _, err := d.db.Exec(`
			UPDATE server_event_outbox
			SET attempt_count = attempt_count + 1, last_error = ?, status = 'pending'
			WHERE event_id = ? AND revision = ?`, message, event.EventID, event.Revision); err != nil {
			return fmt.Errorf("record server event %s failure: %w", event.EventID, err)
		}
	}
	return nil
}

// ServerEventIDs returns the stable keys carried by a staged batch.
func ServerEventKeys(payload ServerEventSyncPayload) []ServerEventKey {
	keys := make([]ServerEventKey, 0, len(payload.Events))
	for _, event := range payload.Events {
		keys = append(keys, ServerEventKey{EventID: event.EventID, Revision: event.Revision})
	}
	return keys
}
