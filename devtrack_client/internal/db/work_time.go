package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/worktime"
)

// WorkActivity is privacy-minimized local evidence used for time inference.
// It intentionally excludes command text, file contents, window state, and
// cloud data.
type WorkActivity struct {
	SourceKind    string
	SourceID      string
	OccurredAt    time.Time
	TicketRef     string
	RepoPath      string
	WorkspaceName string
}

// RecordWorkActivity stores evidence exactly once and attaches it either to a
// matching explicit session or to a deterministic inferred window. The bool is
// false when the evidence was already recorded.
func (d *Database) RecordWorkActivity(activity WorkActivity, policy worktime.Policy) (int64, bool, error) {
	if activity.SourceKind == "" || activity.SourceID == "" || activity.OccurredAt.IsZero() {
		return 0, false, fmt.Errorf("work activity requires source kind, source ID, and timestamp")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, false, fmt.Errorf("begin work activity: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT OR IGNORE INTO work_activity_evidence (
			source_kind, source_id, occurred_at, ticket_ref, repo_path, workspace_name
		) VALUES (?, ?, ?, ?, ?, ?)`,
		activity.SourceKind, activity.SourceID, activity.OccurredAt.Format(time.RFC3339Nano),
		activity.TicketRef, activity.RepoPath, activity.WorkspaceName)
	if err != nil {
		return 0, false, fmt.Errorf("insert work activity: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("inspect work activity insert: %w", err)
	}
	if rows == 0 {
		var existing sql.NullInt64
		if err := tx.QueryRow(`
			SELECT work_session_id FROM work_activity_evidence
			WHERE source_kind = ? AND source_id = ?`, activity.SourceKind, activity.SourceID).Scan(&existing); err != nil {
			return 0, false, fmt.Errorf("read duplicate work activity: %w", err)
		}
		return existing.Int64, false, nil
	}

	if sessionID, ok, err := attachToExplicitSession(tx, activity); err != nil {
		return 0, false, err
	} else if ok {
		if err := linkWorkEvidence(tx, activity, sessionID); err != nil {
			return 0, false, err
		}
		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("commit explicit work activity: %w", err)
		}
		return sessionID, true, nil
	}

	sessionID, err := upsertInferredWindow(tx, activity, policy)
	if err != nil {
		return 0, false, err
	}
	if err := linkWorkEvidence(tx, activity, sessionID); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit inferred work activity: %w", err)
	}
	return sessionID, true, nil
}

func attachToExplicitSession(tx *sql.Tx, activity WorkActivity) (int64, bool, error) {
	var id int64
	var ticketRef, commits string
	var evidenceCount int
	err := tx.QueryRow(`
		SELECT id, COALESCE(ticket_ref, ''), COALESCE(commits, '[]'), evidence_count
		FROM work_sessions
		WHERE ended_at IS NULL AND measurement_source = 'explicit'
		  AND (repo_path = '' OR repo_path = '.' OR repo_path = ?)
		  AND (workspace_name = '' OR workspace_name = ?)
		ORDER BY started_at DESC LIMIT 1`, activity.RepoPath, activity.WorkspaceName).
		Scan(&id, &ticketRef, &commits, &evidenceCount)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find explicit work session: %w", err)
	}
	if ticketRef == "" {
		ticketRef = activity.TicketRef
	}
	commits = appendJSONStringValue(commits, activity.SourceID)
	if _, err := tx.Exec(`
		UPDATE work_sessions
		SET ticket_ref = ?, commits = ?, last_activity_at = ?, evidence_count = ?, confidence = 1.0
		WHERE id = ?`, ticketRef, commits, activity.OccurredAt.Format(time.RFC3339Nano), evidenceCount+1, id); err != nil {
		return 0, false, fmt.Errorf("update explicit work session: %w", err)
	}
	return id, true, nil
}

func upsertInferredWindow(tx *sql.Tx, activity WorkActivity, policy worktime.Policy) (int64, error) {
	var id int64
	var firstRaw, lastRaw, ticketRef, repoPath, workspaceName, commits string
	var evidenceCount int
	err := tx.QueryRow(`
		SELECT id, started_at, last_activity_at, COALESCE(ticket_ref, ''),
		       COALESCE(repo_path, ''), COALESCE(workspace_name, ''),
		       COALESCE(commits, '[]'), evidence_count
		FROM work_sessions
		WHERE measurement_source = 'inferred'
		  AND repo_path = ? AND workspace_name = ?
		ORDER BY last_activity_at DESC LIMIT 1`, activity.RepoPath, activity.WorkspaceName).
		Scan(&id, &firstRaw, &lastRaw, &ticketRef, &repoPath, &workspaceName, &commits, &evidenceCount)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("find inferred work window: %w", err)
	}
	if err == nil {
		first, firstErr := parseWorkSessionTime(firstRaw)
		last, lastErr := parseWorkSessionTime(lastRaw)
		window := worktime.Window{
			FirstActivity: first,
			LastActivity:  last,
			Context: worktime.Context{
				TicketRef: ticketRef, RepoPath: repoPath, WorkspaceName: workspaceName,
			},
			EvidenceCount: evidenceCount,
		}
		context := worktime.Context{
			TicketRef: activity.TicketRef, RepoPath: activity.RepoPath, WorkspaceName: activity.WorkspaceName,
		}
		if firstErr == nil && lastErr == nil && worktime.CanExtend(window, activity.OccurredAt, context, policy) {
			count := evidenceCount + 1
			_, end := worktime.Bounds(first, activity.OccurredAt, policy)
			commits = appendJSONStringValue(commits, activity.SourceID)
			if _, err := tx.Exec(`
				UPDATE work_sessions
				SET ended_at = ?, last_activity_at = ?, duration_minutes = ?,
				    commits = ?, evidence_count = ?, confidence = ?
				WHERE id = ?`, end.Format(time.RFC3339Nano), activity.OccurredAt.Format(time.RFC3339Nano),
				worktime.Minutes(first, activity.OccurredAt, policy), commits, count,
				worktime.Confidence(count), id); err != nil {
				return 0, fmt.Errorf("extend inferred work window: %w", err)
			}
			return id, nil
		}
	}

	start, end := worktime.Bounds(activity.OccurredAt, activity.OccurredAt, policy)
	result, err := tx.Exec(`
		INSERT INTO work_sessions (
			started_at, ended_at, ticket_ref, repo_path, workspace_name, commits,
			duration_minutes, measurement_source, last_activity_at, confidence, evidence_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'inferred', ?, ?, 1)`,
		start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), activity.TicketRef,
		activity.RepoPath, activity.WorkspaceName, buildJSONStringArray([]string{activity.SourceID}),
		worktime.Minutes(activity.OccurredAt, activity.OccurredAt, policy),
		activity.OccurredAt.Format(time.RFC3339Nano), worktime.Confidence(1))
	if err != nil {
		return 0, fmt.Errorf("create inferred work window: %w", err)
	}
	return result.LastInsertId()
}

func linkWorkEvidence(tx *sql.Tx, activity WorkActivity, sessionID int64) error {
	if _, err := tx.Exec(`
		UPDATE work_activity_evidence SET work_session_id = ?
		WHERE source_kind = ? AND source_id = ?`, sessionID, activity.SourceKind, activity.SourceID); err != nil {
		return fmt.Errorf("link work activity evidence: %w", err)
	}
	return nil
}

func appendJSONStringValue(current, value string) string {
	var values []string
	if len(current) >= 2 && current[0] == '[' && current[len(current)-1] == ']' {
		inner := current[1 : len(current)-1]
		if inner != "" {
			values = append(values, splitJSONStringArray(inner)...)
		}
	}
	values = append(values, value)
	return buildJSONStringArray(values)
}

func parseWorkSessionTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid work session time %q", value)
}
