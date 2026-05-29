package github

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

const createGitHubTable = `
CREATE TABLE IF NOT EXISTS github_issues (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	number      INTEGER NOT NULL,
	repo        TEXT NOT NULL,
	title       TEXT NOT NULL,
	body        TEXT,
	url         TEXT,
	state       TEXT,
	labels      TEXT,
	updated_at  TEXT,
	synced_at   DATETIME,
	UNIQUE(number, repo)
)`

// Sync fetches all assigned open issues and upserts them into the github_issues SQLite table.
func Sync(db *sql.DB) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is not set")
	}
	username := os.Getenv("GITHUB_USERNAME")

	// Ensure table exists
	if _, err := db.Exec(createGitHubTable); err != nil {
		return fmt.Errorf("github sync: create table: %w", err)
	}

	issues, err := ListIssues(token, username)
	if err != nil {
		return fmt.Errorf("github sync: list issues: %w", err)
	}

	now := time.Now().UTC()
	upserted := 0
	for _, iss := range issues {
		_, err := db.Exec(`
			INSERT INTO github_issues (number, repo, title, body, url, state, labels, updated_at, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(number, repo) DO UPDATE SET
				title      = excluded.title,
				body       = excluded.body,
				url        = excluded.url,
				state      = excluded.state,
				labels     = excluded.labels,
				updated_at = excluded.updated_at,
				synced_at  = excluded.synced_at`,
			iss.Number, iss.Repo, iss.Title, iss.Body,
			iss.URL, iss.State, iss.LabelNames(), iss.UpdatedAt, now)
		if err != nil {
			return fmt.Errorf("github sync: upsert #%d: %w", iss.Number, err)
		}
		upserted++
	}

	fmt.Printf("github sync: %d issues synced to SQLite\n", upserted)
	return nil
}

// ListCached returns issues stored in SQLite without hitting the API.
func ListCached(db *sql.DB) ([]Issue, error) {
	if _, err := db.Exec(createGitHubTable); err != nil {
		return nil, fmt.Errorf("github cached: create table: %w", err)
	}

	rows, err := db.Query(`
		SELECT number, repo, title, body, url, state, labels, updated_at
		FROM github_issues ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var iss Issue
		var labelsStr string
		if err := rows.Scan(&iss.Number, &iss.Repo, &iss.Title, &iss.Body,
			&iss.URL, &iss.State, &labelsStr, &iss.UpdatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, iss)
	}
	return issues, rows.Err()
}
