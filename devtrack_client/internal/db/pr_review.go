package db

import (
	"database/sql"
	"fmt"
	"time"
)

// PRReviewComment represents a review comment on a developer-authored PR.
// Populated by the alert poller when a reviewer leaves a comment; status is
// updated to "classified" by the infra layer after calling /review/classify.
type PRReviewComment struct {
	Platform     string    // "github" | "azure" | "gitlab"
	CommentID    string    // platform-native comment ID (primary key component)
	PRID         string    // PR number or platform PR ID
	Workspace    string    // workspace name from workspaces.yaml
	Status       string    // new | classified | fix_applied | escalated | done
	CommentBody  string    // full text of the review comment
	ClassifiedAs string    // "auto_fixable" | "needs_human" | "" (not yet classified)
	FixHint      string    // short imperative fix instruction (populated for auto_fixable)
	AttemptCount int       // number of fix-loop attempts made for this comment (migration 013)
	CommentURL   string    // web URL of the comment (not stored in DB; populated by alerter)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InsertPRReviewComment inserts a new pr_review_comments row.
// Returns an error if the (platform, comment_id) pair already exists.
func (d *Database) InsertPRReviewComment(c PRReviewComment) error {
	query := `
		INSERT INTO pr_review_comments
			(platform, comment_id, pr_id, workspace, status, comment_body, classified_as, fix_hint)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	status := c.Status
	if status == "" {
		status = "new"
	}
	_, err := d.db.Exec(query,
		c.Platform,
		c.CommentID,
		c.PRID,
		c.Workspace,
		status,
		c.CommentBody,
		c.ClassifiedAs,
		c.FixHint,
	)
	if err != nil {
		return fmt.Errorf("InsertPRReviewComment: %w", err)
	}
	return nil
}

// GetPRReviewComment retrieves a single row by (platform, comment_id).
// Returns nil, nil when no matching row exists.
func (d *Database) GetPRReviewComment(platform, commentID string) (*PRReviewComment, error) {
	query := `
		SELECT platform, comment_id, pr_id, workspace, status, comment_body,
		       COALESCE(classified_as, ''), fix_hint, created_at, updated_at,
		       COALESCE(attempt_count, 0)
		FROM pr_review_comments
		WHERE platform = ? AND comment_id = ?
	`
	row := d.db.QueryRow(query, platform, commentID)
	c, err := scanPRReviewComment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetPRReviewComment(%s, %s): %w", platform, commentID, err)
	}
	return c, nil
}

// UpdatePRReviewCommentStatus updates the status, classified_as, and fix_hint columns
// for the given (platform, comment_id), and stamps updated_at = datetime('now').
// classifiedAs and fixHint may be empty strings when the status is still "new".
func (d *Database) UpdatePRReviewCommentStatus(platform, commentID, status, classifiedAs, fixHint string) error {
	query := `
		UPDATE pr_review_comments
		SET status        = ?,
		    classified_as = ?,
		    fix_hint      = ?,
		    updated_at    = datetime('now')
		WHERE platform = ? AND comment_id = ?
	`
	_, err := d.db.Exec(query, status, classifiedAs, fixHint, platform, commentID)
	if err != nil {
		return fmt.Errorf("UpdatePRReviewCommentStatus(%s, %s): %w", platform, commentID, err)
	}
	return nil
}

// ListPRReviewCommentsByPR returns all comments for a given (platform, pr_id) pair,
// ordered by created_at ASC.
func (d *Database) ListPRReviewCommentsByPR(platform, prID string) ([]PRReviewComment, error) {
	query := `
		SELECT platform, comment_id, pr_id, workspace, status, comment_body,
		       COALESCE(classified_as, ''), fix_hint, created_at, updated_at,
		       COALESCE(attempt_count, 0)
		FROM pr_review_comments
		WHERE platform = ? AND pr_id = ?
		ORDER BY created_at ASC
	`
	rows, err := d.db.Query(query, platform, prID)
	if err != nil {
		return nil, fmt.Errorf("ListPRReviewCommentsByPR(%s, %s): %w", platform, prID, err)
	}
	defer rows.Close()
	return scanPRReviewComments(rows)
}

// ListPRReviewCommentsByStatus returns all comments with the given status,
// ordered by created_at ASC. Pass "" to return all statuses.
func (d *Database) ListPRReviewCommentsByStatus(status string) ([]PRReviewComment, error) {
	var (
		query string
		args  []any
	)
	if status == "" {
		query = `
			SELECT platform, comment_id, pr_id, workspace, status, comment_body,
			       COALESCE(classified_as, ''), fix_hint, created_at, updated_at,
			       COALESCE(attempt_count, 0)
			FROM pr_review_comments
			ORDER BY created_at ASC
		`
	} else {
		query = `
			SELECT platform, comment_id, pr_id, workspace, status, comment_body,
			       COALESCE(classified_as, ''), fix_hint, created_at, updated_at,
			       COALESCE(attempt_count, 0)
			FROM pr_review_comments
			WHERE status = ?
			ORDER BY created_at ASC
		`
		args = []any{status}
	}
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListPRReviewCommentsByStatus(%q): %w", status, err)
	}
	defer rows.Close()
	return scanPRReviewComments(rows)
}

// ListPRReviewCommentsRecent returns all pr_review_comments created or updated
// within the last N hours, ordered by created_at ASC.
// Used by "devtrack review status" to build the per-PR activity summary.
func (d *Database) ListPRReviewCommentsRecent(hours int) ([]PRReviewComment, error) {
	query := `
		SELECT platform, comment_id, pr_id, workspace, status, comment_body,
		       COALESCE(classified_as, ''), fix_hint, created_at, updated_at,
		       COALESCE(attempt_count, 0)
		FROM pr_review_comments
		WHERE created_at >= datetime('now', ? || ' hours')
		ORDER BY created_at ASC
	`
	// SQLite modifier requires a negative number for "N hours ago".
	modifier := fmt.Sprintf("-%d", hours)
	rows, err := d.db.Query(query, modifier)
	if err != nil {
		return nil, fmt.Errorf("ListPRReviewCommentsRecent: %w", err)
	}
	defer rows.Close()
	return scanPRReviewComments(rows)
}

// IncrementPRReviewCommentAttempts increments attempt_count for (platform, commentID)
// and stamps updated_at = datetime('now'). Returns the new attempt_count value.
func (d *Database) IncrementPRReviewCommentAttempts(platform, commentID string) (int, error) {
	_, err := d.db.Exec(`
		UPDATE pr_review_comments
		SET attempt_count = attempt_count + 1,
		    updated_at    = datetime('now')
		WHERE platform = ? AND comment_id = ?
	`, platform, commentID)
	if err != nil {
		return 0, fmt.Errorf("IncrementPRReviewCommentAttempts(%s, %s): %w", platform, commentID, err)
	}

	var count int
	if err := d.db.QueryRow(`
		SELECT COALESCE(attempt_count, 0) FROM pr_review_comments
		WHERE platform = ? AND comment_id = ?
	`, platform, commentID).Scan(&count); err != nil {
		return 0, fmt.Errorf("IncrementPRReviewCommentAttempts select(%s, %s): %w", platform, commentID, err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Internal scan helpers
// ---------------------------------------------------------------------------

type prReviewCommentScanner interface {
	Scan(dest ...any) error
}

func scanPRReviewComment(row prReviewCommentScanner) (*PRReviewComment, error) {
	var c PRReviewComment
	var createdAt, updatedAt string
	err := row.Scan(
		&c.Platform,
		&c.CommentID,
		&c.PRID,
		&c.Workspace,
		&c.Status,
		&c.CommentBody,
		&c.ClassifiedAs,
		&c.FixHint,
		&createdAt,
		&updatedAt,
		&c.AttemptCount,
	)
	if err != nil {
		return nil, err
	}
	c.CreatedAt = parseSQLiteTime(createdAt)
	c.UpdatedAt = parseSQLiteTime(updatedAt)
	return &c, nil
}

func scanPRReviewComments(rows *sql.Rows) ([]PRReviewComment, error) {
	var comments []PRReviewComment
	for rows.Next() {
		var c PRReviewComment
		var createdAt, updatedAt string
		err := rows.Scan(
			&c.Platform,
			&c.CommentID,
			&c.PRID,
			&c.Workspace,
			&c.Status,
			&c.CommentBody,
			&c.ClassifiedAs,
			&c.FixHint,
			&createdAt,
			&updatedAt,
			&c.AttemptCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pr_review_comments row: %w", err)
		}
		c.CreatedAt = parseSQLiteTime(createdAt)
		c.UpdatedAt = parseSQLiteTime(updatedAt)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
