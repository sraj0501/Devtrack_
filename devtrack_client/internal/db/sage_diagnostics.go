package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SageMaxAttempts bounds automatic background retries. Exhausted work remains
// durable and visible to doctor/log instead of spinning forever.
const SageMaxAttempts = 5

// SageDiagnostics is a privacy-safe summary of every durable pipeline state.
// Counts are per normalized command signature except Captured, which counts
// append-only source events.
type SageDiagnostics struct {
	Captured          int `json:"captured"`
	Waiting           int `json:"waiting"`
	Retrying          int `json:"retrying"`
	Processing        int `json:"processing"`
	ExpiredLeases     int `json:"expired_leases"`
	Distilled         int `json:"distilled"`
	Skipped           int `json:"skipped"`
	PublicationQueued int `json:"publication_queued"`
	PublicationRetry  int `json:"publication_retrying"`
	Documented        int `json:"documented"`
	TerminalFailed    int `json:"terminally_failed"`
}

// SageLogEntry deliberately excludes commands, raw signatures, model output,
// paths, and timestamps derived from captured content.
type SageLogEntry struct {
	ID               string `json:"id"`
	State            string `json:"state"`
	Attempts         int    `json:"attempts"`
	NextRetryMS      int64  `json:"next_retry_ms,omitempty"`
	LeaseUntilMS     int64  `json:"lease_until_ms,omitempty"`
	Diagnostic       string `json:"diagnostic,omitempty"`
	SkipReason       string `json:"skip_reason,omitempty"`
	PublicationState string `json:"publication_state,omitempty"`
	Documented       bool   `json:"documented"`
}

func sageDiagnosticID(signature string) string {
	digest := sha256.Sum256([]byte(signature))
	return hex.EncodeToString(digest[:6])
}

func (d *Database) SageDiagnostics(ctx context.Context, now time.Time) (SageDiagnostics, error) {
	var out SageDiagnostics
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sage_events`).Scan(&out.Captured); err != nil {
		return out, fmt.Errorf("count captured Sage events: %w", err)
	}
	rows, err := d.db.QueryContext(ctx, `SELECT state,attempts,lease_until_ms FROM sage_jobs`)
	if err != nil {
		return out, fmt.Errorf("summarize Sage jobs: %w", err)
	}
	for rows.Next() {
		var state string
		var attempts int
		var leaseUntil int64
		if err := rows.Scan(&state, &attempts, &leaseUntil); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan Sage job summary: %w", err)
		}
		switch {
		case (state == "retrying" || state == "processing") && attempts >= SageMaxAttempts:
			out.TerminalFailed++
		case state == "waiting":
			out.Waiting++
		case state == "retrying":
			out.Retrying++
		case state == "processing":
			out.Processing++
			if leaseUntil <= now.UnixMilli() {
				out.ExpiredLeases++
			}
		case state == "distilled":
			out.Distilled++
		case state == "skipped":
			out.Skipped++
		}
	}
	if err := rows.Close(); err != nil {
		return out, fmt.Errorf("close Sage job summary: %w", err)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("read Sage job summary: %w", err)
	}
	rows, err = d.db.QueryContext(ctx, `SELECT state,attempts FROM sage_publications`)
	if err != nil {
		return out, fmt.Errorf("summarize Sage publications: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var attempts int
		if err := rows.Scan(&state, &attempts); err != nil {
			return out, fmt.Errorf("scan Sage publication summary: %w", err)
		}
		switch {
		case state == "retrying" && attempts >= SageMaxAttempts:
			out.TerminalFailed++
		case state == "waiting":
			out.PublicationQueued++
		case state == "retrying":
			out.PublicationRetry++
		case state == "documented":
			out.Documented++
		}
	}
	return out, rows.Err()
}

func (d *Database) SageLog(ctx context.Context, limit int) ([]SageLogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.db.QueryContext(ctx, `SELECT j.signature,j.state,j.attempts,j.next_retry_ms,
		j.lease_until_ms,j.last_error,j.skip_reason,COALESCE(p.state,''),COALESCE(p.attempts,0)
		FROM sage_jobs j LEFT JOIN sage_publications p ON p.signature=j.signature
		ORDER BY MAX(j.completed_ms,j.next_retry_ms,j.lease_until_ms) DESC,j.signature LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read Sage log: %w", err)
	}
	defer rows.Close()
	var entries []SageLogEntry
	for rows.Next() {
		var signature string
		var publicationAttempts int
		var entry SageLogEntry
		if err := rows.Scan(&signature, &entry.State, &entry.Attempts, &entry.NextRetryMS,
			&entry.LeaseUntilMS, &entry.Diagnostic, &entry.SkipReason, &entry.PublicationState,
			&publicationAttempts); err != nil {
			return nil, fmt.Errorf("scan Sage log: %w", err)
		}
		entry.ID = sageDiagnosticID(signature)
		entry.Diagnostic = safeSageDiagnostic(entry.Diagnostic)
		entry.SkipReason = safeSageSkipReason(entry.SkipReason)
		entry.Documented = entry.PublicationState == "documented"
		if (entry.State == "retrying" || entry.State == "processing") && entry.Attempts >= SageMaxAttempts {
			entry.State = "terminally_failed"
		}
		if entry.PublicationState == "retrying" {
			if publicationAttempts >= SageMaxAttempts {
				entry.State = "terminally_failed"
				entry.Diagnostic = "knowledge publication failure"
			}
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func safeSageDiagnostic(value string) string {
	switch value {
	case "", "model timeout", "model token limit", "model or persistence failure", "knowledge publication failure":
		return value
	default:
		return "processing failure"
	}
}

func safeSageSkipReason(value string) string {
	switch value {
	case "", "model judged command not useful":
		return value
	default:
		return "explicit skip"
	}
}
