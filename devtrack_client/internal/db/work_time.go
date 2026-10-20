package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	if activity.RepoPath != "" {
		activity.RepoPath = filepath.Clean(activity.RepoPath)
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
	var ticketRef, commits, startedRaw, lastRaw string
	var evidenceCount int
	err := tx.QueryRow(`
		SELECT id, COALESCE(ticket_ref, ''), COALESCE(commits, '[]'), evidence_count, started_at, COALESCE(last_activity_at, started_at)
		FROM work_sessions
		WHERE ended_at IS NULL AND measurement_source = 'explicit'
		  AND (repo_path = '' OR repo_path = '.' OR repo_path = ?)
		  AND (workspace_name = '' OR workspace_name = ?)
		ORDER BY started_at DESC LIMIT 1`, activity.RepoPath, activity.WorkspaceName).
		Scan(&id, &ticketRef, &commits, &evidenceCount, &startedRaw, &lastRaw)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find explicit work session: %w", err)
	}
	started, err := parseWorkSessionTime(startedRaw)
	if err != nil {
		return 0, false, err
	}
	if activity.OccurredAt.Before(started) {
		return 0, false, nil
	}
	last, err := parseWorkSessionTime(lastRaw)
	if err != nil {
		return 0, false, err
	}
	// A newer contradictory ticket ends explicit attribution at its last evidence.
	// Late commits must never close or rewind a currently running session.
	if ticketRef != "" && ticketRef != activity.TicketRef {
		if !activity.OccurredAt.Before(last) {
			_, err := tx.Exec(`UPDATE work_sessions SET ended_at=?, duration_minutes=?, auto_stopped=1 WHERE id=?`,
				last.Format(time.RFC3339Nano), int(last.Sub(started)/time.Minute), id)
			return 0, false, err
		}
		return 0, false, nil
	}
	if activity.OccurredAt.After(last) {
		last = activity.OccurredAt
	}
	if ticketRef == "" {
		ticketRef = activity.TicketRef
	}
	commits = appendJSONStringValue(commits, activity.SourceID)
	if _, err := tx.Exec(`
		UPDATE work_sessions
		SET ticket_ref = ?, commits = ?, last_activity_at = ?, evidence_count = ?, confidence = 1.0
		WHERE id = ?`, ticketRef, commits, last.Format(time.RFC3339Nano), evidenceCount+1, id); err != nil {
		return 0, false, fmt.Errorf("update explicit work session: %w", err)
	}
	return id, true, nil
}

func upsertInferredWindow(tx *sql.Tx, activity WorkActivity, policy worktime.Policy) (int64, error) {
	var id int64
	var firstRaw, lastRaw, ticketRef, repoPath, workspaceName, commits, startRaw string
	var evidenceCount int
	err := tx.QueryRow(`
		SELECT id, (SELECT occurred_at FROM work_activity_evidence
                    WHERE work_session_id = work_sessions.id ORDER BY id LIMIT 1), last_activity_at, COALESCE(ticket_ref, ''),
		       COALESCE(repo_path, ''), COALESCE(workspace_name, ''),
		       COALESCE(commits, '[]'), evidence_count, started_at
		FROM work_sessions
		WHERE repo_path = ? AND workspace_name = ?
          AND measurement_source = 'inferred'
          AND id = (SELECT MAX(id) FROM work_sessions WHERE repo_path = ? AND workspace_name = ?)
		ORDER BY id DESC LIMIT 1`, activity.RepoPath, activity.WorkspaceName, activity.RepoPath, activity.WorkspaceName).
		Scan(&id, &firstRaw, &lastRaw, &ticketRef, &repoPath, &workspaceName, &commits, &evidenceCount, &startRaw)
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
			start, err := parseWorkSessionTime(startRaw)
			if err != nil {
				return 0, err
			}
			_, end := worktime.Bounds(first, activity.OccurredAt, policy)
			clippedStart, clippedEnd, err := clipWorkWindow(tx, activity, id, start, end)
			if err != nil {
				return 0, err
			}
			if !clippedStart.Equal(start) || clippedEnd.Before(activity.OccurredAt) {
				return insertInferredWindow(tx, activity, policy)
			}
			end = clippedEnd
			commits = appendJSONStringValue(commits, activity.SourceID)
			if _, err := tx.Exec(`
				UPDATE work_sessions
				SET ended_at = ?, last_activity_at = ?, duration_minutes = ?,
				    commits = ?, evidence_count = ?, confidence = ?
				WHERE id = ?`, end.Format(time.RFC3339Nano), activity.OccurredAt.Format(time.RFC3339Nano),
				int(end.Sub(start)/time.Minute), commits, count,
				worktime.Confidence(count), id); err != nil {
				return 0, fmt.Errorf("extend inferred work window: %w", err)
			}
			return id, nil
		}
	}

	return insertInferredWindow(tx, activity, policy)
}

// clipWorkWindow clips padding and extension against previously allocated time.
// Fully covered observations retain evidence with zero additional minutes.
// An active explicit session reserves all time after its start.
func clipWorkWindow(tx *sql.Tx, activity WorkActivity, exclude int64, start, end time.Time) (time.Time, time.Time, error) {
	rows, err := tx.Query(`SELECT started_at, ended_at FROM work_sessions
        WHERE id != ? AND (repo_path = ? OR repo_path = '' OR repo_path = '.')
        AND (workspace_name = ? OR workspace_name = '')`, exclude, activity.RepoPath, activity.WorkspaceName)
	if err != nil {
		return start, end, err
	}
	defer rows.Close()
	for rows.Next() {
		var fromRaw string
		var toRaw sql.NullString
		if err := rows.Scan(&fromRaw, &toRaw); err != nil {
			return start, end, err
		}
		from, err := parseWorkSessionTime(fromRaw)
		if err != nil {
			return start, end, err
		}
		to := end
		if toRaw.Valid {
			to, err = parseWorkSessionTime(toRaw.String)
			if err != nil {
				return start, end, err
			}
			if !to.After(from) {
				continue
			}
		}
		if !activity.OccurredAt.Before(from) && (!toRaw.Valid || activity.OccurredAt.Before(to)) {
			if !toRaw.Valid || !to.Before(end) {
				return activity.OccurredAt, activity.OccurredAt, nil
			}
			if to.After(start) {
				start = to
			}
		}
		if !to.After(activity.OccurredAt) && to.After(start) {
			start = to
		}
		if from.After(activity.OccurredAt) && from.Before(end) {
			end = from
		}
	}
	return start, end, rows.Err()
}

func insertInferredWindow(tx *sql.Tx, activity WorkActivity, policy worktime.Policy) (int64, error) {
	start, end := worktime.Bounds(activity.OccurredAt, activity.OccurredAt, policy)
	start, end, err := clipWorkWindow(tx, activity, 0, start, end)
	if err != nil {
		return 0, err
	}

	result, err := tx.Exec(`
		INSERT INTO work_sessions (
			started_at, ended_at, ticket_ref, repo_path, workspace_name, commits,
			duration_minutes, measurement_source, last_activity_at, confidence, evidence_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'inferred', ?, ?, 1)`,
		start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), activity.TicketRef,
		activity.RepoPath, activity.WorkspaceName, buildJSONStringArray([]string{activity.SourceID}),
		int(end.Sub(start)/time.Minute),
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

// CloseInactiveWorkSession closes at the last observed activity, never at the
// scheduler's clock. A start with no subsequent evidence measures zero minutes.
// A zero idle duration is the unconditional EOD closure mode.
func (d *Database) CloseInactiveWorkSession(now time.Time, idle time.Duration) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var startRaw, lastRaw string
	err = tx.QueryRow(`SELECT id, started_at, COALESCE(last_activity_at, started_at)
 FROM work_sessions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&id, &startRaw, &lastRaw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	start, err := parseWorkSessionTime(startRaw)
	if err != nil {
		return err
	}
	last, err := parseWorkSessionTime(lastRaw)
	if err != nil {
		return err
	}
	if last.Before(start) {
		last = start
	}
	if now.Before(last) || (idle > 0 && now.Sub(last) < idle) {
		return nil
	}
	minutes := int(last.Sub(start) / time.Minute)
	if _, err = tx.Exec(`UPDATE work_sessions SET ended_at = ?, duration_minutes = ?, auto_stopped = 1 WHERE id = ?`, last.Format(time.RFC3339Nano), minutes, id); err != nil {
		return err
	}
	return tx.Commit()
}

// insertWorkSessionAt makes the start boundary atomic with removal of inferred
// trailing padding. Evidence and manual adjustments are retained.
func (d *Database) insertWorkSessionAt(ticketRef, repoPath, workspace string, now time.Time) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM work_sessions WHERE ended_at IS NULL`).Scan(&active); err != nil {
		return 0, err
	}
	if active != 0 {
		return 0, fmt.Errorf("a work session is already active")
	}
	repoPath = filepath.Clean(repoPath)
	rows, err := tx.Query(`SELECT id, started_at, ended_at, last_activity_at, measurement_source
        FROM work_sessions WHERE repo_path=? AND workspace_name=?`, repoPath, workspace)
	if err != nil {
		return 0, err
	}
	type trim struct {
		id      int64
		minutes int
	}
	var trims []trim
	for rows.Next() {
		var id int64
		var startRaw, endRaw, source string
		var lastRaw sql.NullString
		if err := rows.Scan(&id, &startRaw, &endRaw, &lastRaw, &source); err != nil {
			rows.Close()
			return 0, err
		}
		start, e1 := parseWorkSessionTime(startRaw)
		end, e2 := parseWorkSessionTime(endRaw)
		if e1 != nil || e2 != nil {
			rows.Close()
			return 0, fmt.Errorf("invalid session %d interval", id)
		}
		if !end.After(now) {
			continue
		}
		last, e3 := parseWorkSessionTime(lastRaw.String)
		if source != "inferred" || e3 != nil || last.After(now) || start.After(now) {
			rows.Close()
			return 0, fmt.Errorf("session %d has future activity; check the system clock before starting", id)
		}
		trims = append(trims, trim{id, int(now.Sub(start) / time.Minute)})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range trims {
		if _, err := tx.Exec(`UPDATE work_sessions SET ended_at=?, duration_minutes=? WHERE id=?`, now.Format(time.RFC3339Nano), item.minutes, item.id); err != nil {
			return 0, err
		}
	}
	result, err := tx.Exec(`INSERT INTO work_sessions
        (started_at, ticket_ref, repo_path, workspace_name, commits, measurement_source, confidence)
        VALUES (?, ?, ?, ?, '[]', 'explicit', 1.0)`, now.Format(time.RFC3339Nano), ticketRef, repoPath, workspace)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}
