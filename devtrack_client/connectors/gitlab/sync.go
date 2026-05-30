package gitlab

import (
	"database/sql"
	"fmt"
	"time"
)

const createGitLabTable = `
CREATE TABLE IF NOT EXISTS gitlab_issues (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	iid         INTEGER NOT NULL,
	repo        TEXT NOT NULL,
	title       TEXT NOT NULL,
	body        TEXT,
	url         TEXT,
	state       TEXT,
	labels      TEXT,
	updated_at  TEXT,
	synced_at   DATETIME,
	UNIQUE(iid, repo)
)`

// Sync fetches all assigned open issues and upserts them into gitlab_issues.
// username comes from workspaces.yaml pm_username; pass "" to auto-detect.
func (c *Client) Sync(db *sql.DB, username string) error {
	if _, err := db.Exec(createGitLabTable); err != nil {
		return fmt.Errorf("gitlab sync: create table: %w", err)
	}

	issues, err := c.ListIssues(username)
	if err != nil {
		return fmt.Errorf("gitlab sync: list issues: %w", err)
	}

	now := time.Now().UTC()
	upserted := 0
	for _, iss := range issues {
		_, err := db.Exec(`
			INSERT INTO gitlab_issues (iid, repo, title, body, url, state, labels, updated_at, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(iid, repo) DO UPDATE SET
				title      = excluded.title,
				body       = excluded.body,
				url        = excluded.url,
				state      = excluded.state,
				labels     = excluded.labels,
				updated_at = excluded.updated_at,
				synced_at  = excluded.synced_at`,
			iss.IID, iss.Repo, iss.Title, iss.Body,
			iss.URL, iss.State, iss.LabelNames(), iss.UpdatedAt, now)
		if err != nil {
			return fmt.Errorf("gitlab sync: upsert #%d: %w", iss.IID, err)
		}
		upserted++
	}

	fmt.Printf("gitlab sync: %d issues synced to SQLite\n", upserted)
	return nil
}

// ListCached returns issues stored in SQLite without hitting the API.
func ListCached(db *sql.DB) ([]Issue, error) {
	if _, err := db.Exec(createGitLabTable); err != nil {
		return nil, fmt.Errorf("gitlab cached: create table: %w", err)
	}

	rows, err := db.Query(`
		SELECT iid, repo, title, body, url, state, labels, updated_at
		FROM gitlab_issues ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var iss Issue
		var labelsStr string
		if err := rows.Scan(&iss.IID, &iss.Repo, &iss.Title, &iss.Body,
			&iss.URL, &iss.State, &labelsStr, &iss.UpdatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, iss)
	}
	return issues, rows.Err()
}
