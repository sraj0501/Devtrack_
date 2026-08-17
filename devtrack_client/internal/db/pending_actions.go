package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PendingAction represents a staged outbound PM action awaiting human review or
// auto-approval. Every action that DevTrack intends to post to an external
// system must pass through this table first (Phase 1 non-negotiable).
type PendingAction struct {
	ID         int64
	ActionType string    // e.g. "post_comment", "state_transition", "eod_report"
	Target     string    // e.g. "PROJ-123", "PR #456", "ADO-789"
	Platform   string    // "github", "azure", "gitlab", "jira"
	Workspace  string    // workspace name from workspaces.yaml
	Payload    string    // raw JSON string — callers marshal/unmarshal
	Confidence float64   // 0.0–1.0
	Status     string    // pending | approved | rejected | posted | failed
	ExpiresAt  time.Time // computed from confidence at insert time
	CreatedAt  time.Time
	ActedAt    *time.Time // nullable — set when status changes away from 'pending'
	ActedBy    *string    // nullable — "auto" | "tui" | "cli" | "telegram"
	Error      *string    // nullable — last error if status=failed
}

// validPendingActionStatuses is the set of allowed status values.
var validPendingActionStatuses = map[string]bool{
	"pending":  true,
	"approved": true,
	"rejected": true,
	"posted":   true,
	"failed":   true,
}

// ConfidenceTimeout returns how long an action should wait for human review
// before auto-approval. Logic from PRODUCT_BIBLE.md.
func ConfidenceTimeout(confidence float64, isNewActionType bool) time.Duration {
	if isNewActionType {
		return 30 * time.Minute
	}
	if confidence > 0.90 {
		return 2 * time.Minute
	}
	if confidence >= 0.70 {
		return 5 * time.Minute
	}
	return 15 * time.Minute
}

// InsertPendingAction inserts a new pending action row and returns the new row ID.
func (d *Database) InsertPendingAction(a PendingAction) (int64, error) {
	query := `
		INSERT INTO pending_actions
			(action_type, target, platform, workspace, payload, confidence, status, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := d.db.Exec(query,
		a.ActionType,
		a.Target,
		a.Platform,
		a.Workspace,
		a.Payload,
		a.Confidence,
		a.Status,
		a.ExpiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert pending action: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return id, nil
}

// ListPendingActions returns pending actions ordered by expires_at ASC.
// Pass statusFilter="" to return all statuses, or e.g. "pending" for the queue view.
func (d *Database) ListPendingActions(statusFilter string) ([]PendingAction, error) {
	var (
		query string
		args  []any
	)
	if statusFilter == "" {
		query = `
			SELECT id, action_type, target, platform, workspace, payload, confidence,
			       status, expires_at, created_at, acted_at, acted_by, error
			FROM pending_actions
			ORDER BY expires_at ASC
		`
	} else {
		query = `
			SELECT id, action_type, target, platform, workspace, payload, confidence,
			       status, expires_at, created_at, acted_at, acted_by, error
			FROM pending_actions
			WHERE status = ?
			ORDER BY expires_at ASC
		`
		args = []any{statusFilter}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending actions: %w", err)
	}
	defer rows.Close()

	var actions []PendingAction
	for rows.Next() {
		a, err := scanPendingAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// ListPendingActionsRecent returns all pending actions (any status) created within
// the last N hours, ordered by expires_at ASC. Used by the TUI queue panel.
func (d *Database) ListPendingActionsRecent(hours int) ([]PendingAction, error) {
	query := `
		SELECT id, action_type, target, platform, workspace, payload, confidence,
		       status, expires_at, created_at, acted_at, acted_by, error
		FROM pending_actions
		WHERE created_at >= datetime('now', ? || ' hours')
		ORDER BY expires_at ASC
	`
	// SQLite modifier requires a negative number for "N hours ago".
	modifier := fmt.Sprintf("-%d", hours)
	rows, err := d.db.Query(query, modifier)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent pending actions: %w", err)
	}
	defer rows.Close()

	var actions []PendingAction
	for rows.Next() {
		a, err := scanPendingAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// GetPendingAction retrieves a single pending action by its ID.
// Returns nil, nil when no row matches.
func (d *Database) GetPendingAction(id int64) (*PendingAction, error) {
	query := `
		SELECT id, action_type, target, platform, workspace, payload, confidence,
		       status, expires_at, created_at, acted_at, acted_by, error
		FROM pending_actions
		WHERE id = ?
	`
	row := d.db.QueryRow(query, id)
	a, err := scanPendingActionRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending action %d: %w", id, err)
	}
	return a, nil
}

// UpdatePendingActionStatus sets the status, acted_at, and acted_by columns.
// Valid status values: pending | approved | rejected | posted | failed.
func (d *Database) UpdatePendingActionStatus(id int64, status, actedBy string) error {
	if !validPendingActionStatuses[status] {
		return fmt.Errorf("invalid pending action status %q: must be one of pending, approved, rejected, posted, failed", status)
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin pending action status update for id %d: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := `
		UPDATE pending_actions
		SET status = ?, acted_at = datetime('now'), acted_by = ?
		WHERE id = ?
	`
	result, err := tx.Exec(query, status, actedBy, id)
	if err != nil {
		return fmt.Errorf("failed to update pending action status for id %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read pending action status result for id %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("pending action %d not found", id)
	}

	// Rejecting a server-event batch must reject the exact outbox revisions it
	// contains. Otherwise the next queue poll would immediately stage the same
	// developer data again, making the user's rejection meaningless. A later
	// source-row update increments the revision and naturally makes it pending.
	if status == "rejected" {
		var actionType, payloadJSON string
		if err := tx.QueryRow(
			`SELECT action_type, payload FROM pending_actions WHERE id = ?`, id,
		).Scan(&actionType, &payloadJSON); err != nil {
			return fmt.Errorf("read rejected pending action %d: %w", id, err)
		}
		if actionType == ServerEventSyncActionType {
			var payload ServerEventSyncPayload
			if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
				return fmt.Errorf("decode rejected server event action %d: %w", id, err)
			}
			for _, event := range payload.Events {
				if _, err := tx.Exec(`
					UPDATE server_event_outbox
					SET status = 'rejected', last_error = NULL
					WHERE event_id = ? AND revision = ?`, event.EventID, event.Revision); err != nil {
					return fmt.Errorf("reject server event %s: %w", event.EventID, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pending action status update for id %d: %w", id, err)
	}
	return nil
}

// UpdatePendingActionError sets status='failed', records the error message, and
// stamps acted_at. Called by the queue executor when a PM API call fails.
func (d *Database) UpdatePendingActionError(id int64, errMsg string) error {
	query := `
		UPDATE pending_actions
		SET status = 'failed', error = ?, acted_at = datetime('now')
		WHERE id = ?
	`
	_, err := d.db.Exec(query, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to record pending action error for id %d: %w", id, err)
	}
	return nil
}

// RecordPendingActionAttemptError records a transient dispatch error without
// taking the action out of the queue. Transport failures must retain their
// local-first retry semantics; a later queue poll can try again.
func (d *Database) RecordPendingActionAttemptError(id int64, errMsg string) error {
	_, err := d.db.Exec(`
		UPDATE pending_actions
		SET error = ?
		WHERE id = ?`, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to record pending action attempt error for id %d: %w", id, err)
	}
	return nil
}

// UpdatePendingActionPayload updates the payload JSON for a pending action.
// Used by the TUI edit overlay and the CLI edit command before approval.
func (d *Database) UpdatePendingActionPayload(id int64, payload string) error {
	var actionType string
	if err := d.db.QueryRow(
		`SELECT action_type FROM pending_actions WHERE id = ?`, id,
	).Scan(&actionType); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("pending action %d not found", id)
		}
		return fmt.Errorf("read pending action %d before payload update: %w", id, err)
	}
	if actionType == ServerEventSyncActionType {
		return fmt.Errorf("server event sync actions cannot be edited; approve or reject the batch")
	}
	query := `
		UPDATE pending_actions
		SET payload = ?
		WHERE id = ?
	`
	_, err := d.db.Exec(query, payload, id)
	if err != nil {
		return fmt.Errorf("failed to update pending action payload for id %d: %w", id, err)
	}
	return nil
}

// CountPendingActionsRecent returns a summary count of pending_actions by status.
// postedToday and rejectedToday are scoped to the current calendar day (acted_at).
// Used by the CLI status subcommand and Telegram /queue command.
func (d *Database) CountPendingActionsRecent() (pending, postedToday, rejectedToday int, err error) {
	err = d.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('posted','approved') AND date(acted_at) = date('now') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'rejected' AND date(acted_at) = date('now') THEN 1 ELSE 0 END), 0)
		FROM pending_actions
	`).Scan(&pending, &postedToday, &rejectedToday)
	if err != nil {
		err = fmt.Errorf("failed to count pending actions: %w", err)
	}
	return
}

// ---------------------------------------------------------------------------
// Internal scan helpers
// ---------------------------------------------------------------------------

// scanPendingActionRow scans a *sql.Row into a PendingAction.
func scanPendingActionRow(row *sql.Row) (*PendingAction, error) {
	var a PendingAction
	var expiresAt, createdAt string
	var actedAt sql.NullString
	var actedBy sql.NullString
	var errCol sql.NullString

	err := row.Scan(
		&a.ID, &a.ActionType, &a.Target, &a.Platform, &a.Workspace,
		&a.Payload, &a.Confidence, &a.Status,
		&expiresAt, &createdAt,
		&actedAt, &actedBy, &errCol,
	)
	if err != nil {
		return nil, err
	}
	a.ExpiresAt = parseSQLiteTime(expiresAt)
	a.CreatedAt = parseSQLiteTime(createdAt)
	if actedAt.Valid {
		t := parseSQLiteTime(actedAt.String)
		a.ActedAt = &t
	}
	if actedBy.Valid {
		s := actedBy.String
		a.ActedBy = &s
	}
	if errCol.Valid {
		s := errCol.String
		a.Error = &s
	}
	return &a, nil
}

// scanPendingAction scans the current row of a *sql.Rows into a PendingAction.
func scanPendingAction(rows *sql.Rows) (PendingAction, error) {
	var a PendingAction
	var expiresAt, createdAt string
	var actedAt sql.NullString
	var actedBy sql.NullString
	var errCol sql.NullString

	err := rows.Scan(
		&a.ID, &a.ActionType, &a.Target, &a.Platform, &a.Workspace,
		&a.Payload, &a.Confidence, &a.Status,
		&expiresAt, &createdAt,
		&actedAt, &actedBy, &errCol,
	)
	if err != nil {
		return PendingAction{}, fmt.Errorf("failed to scan pending action: %w", err)
	}
	a.ExpiresAt = parseSQLiteTime(expiresAt)
	a.CreatedAt = parseSQLiteTime(createdAt)
	if actedAt.Valid {
		t := parseSQLiteTime(actedAt.String)
		a.ActedAt = &t
	}
	if actedBy.Valid {
		s := actedBy.String
		a.ActedBy = &s
	}
	if errCol.Valid {
		s := errCol.String
		a.Error = &s
	}
	return a, nil
}

// parseSQLiteTime parses the common SQLite datetime string formats.
func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
