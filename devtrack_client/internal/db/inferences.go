package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Structs — Phase 6 dialectic self-improvement data layer
// ---------------------------------------------------------------------------

// Inference represents a reasoned statement about the developer's style or patterns.
// Evidence is stored as a raw JSON array of trigger/action IDs; callers marshal/unmarshal.
type Inference struct {
	ID          int64
	ContextType string    // commit | comment | report | task | ticket_mapping
	Subject     string    // what the inference is about (e.g. "commit tone")
	Inference   string    // the reasoned statement
	Evidence    string    // raw JSON: []int64 of trigger/action IDs
	Confidence  float64   // 0.0–1.0; updated by corrections
	Source      string    // hermes3 | manual
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Correction represents a developer correction on an inference.
// Weight multiplies this signal relative to ordinary evidence.
type Correction struct {
	ID          int64
	InferenceID int64
	Correction  string  // what the developer said was wrong / the right value
	FlaggedFrom string  // tui | cli | telegram
	Weight      float64 // multiplier on this signal vs ordinary evidence
	CreatedAt   time.Time
}

// ConfidenceThreshold tracks per-action-type adaptive auto-approve thresholds.
// Workspace="" means global (applies to all workspaces).
// Threshold = 0.70 + 0.20 * (approvals / (approvals + rejections)), capped at 0.95.
type ConfidenceThreshold struct {
	ID          int64
	ActionType  string
	Workspace   string
	Threshold   float64
	Approvals   int
	Rejections  int
	LastUpdated time.Time
}

// ---------------------------------------------------------------------------
// Inference CRUD
// ---------------------------------------------------------------------------

// InsertInference inserts a new inference row and returns the new row ID.
func (d *Database) InsertInference(inf Inference) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO inferences (context_type, subject, inference, evidence, confidence, source)
		VALUES (?, ?, ?, ?, ?, ?)
	`, inf.ContextType, inf.Subject, inf.Inference, inf.Evidence, inf.Confidence, inf.Source)
	if err != nil {
		return 0, fmt.Errorf("InsertInference: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("InsertInference LastInsertId: %w", err)
	}
	return id, nil
}

// GetInference retrieves a single inference by ID.
// Returns sql.ErrNoRows (wrapped) when the ID does not exist.
func (d *Database) GetInference(id int64) (*Inference, error) {
	var inf Inference
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, context_type, subject, inference, evidence, confidence, source, created_at, updated_at
		FROM inferences
		WHERE id = ?
	`, id).Scan(
		&inf.ID, &inf.ContextType, &inf.Subject, &inf.Inference,
		&inf.Evidence, &inf.Confidence, &inf.Source,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("GetInference: id %d not found: %w", id, err)
	}
	if err != nil {
		return nil, fmt.Errorf("GetInference: %w", err)
	}
	inf.CreatedAt = parseTimestamp(createdAt)
	inf.UpdatedAt = parseTimestamp(updatedAt)
	return &inf, nil
}

// ListInferences returns up to limit inferences, optionally filtered by contextType.
// Pass contextType="" to return all context types. Results are ordered newest-first.
func (d *Database) ListInferences(contextType string, limit int) ([]Inference, error) {
	var rows *sql.Rows
	var err error
	if contextType == "" {
		rows, err = d.db.Query(`
			SELECT id, context_type, subject, inference, evidence, confidence, source, created_at, updated_at
			FROM inferences
			ORDER BY created_at DESC
			LIMIT ?
		`, limit)
	} else {
		rows, err = d.db.Query(`
			SELECT id, context_type, subject, inference, evidence, confidence, source, created_at, updated_at
			FROM inferences
			WHERE context_type = ?
			ORDER BY created_at DESC
			LIMIT ?
		`, contextType, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("ListInferences: %w", err)
	}
	defer rows.Close()
	return scanInferences(rows)
}

// SearchInferences performs a full-text search over the inferences_fts virtual table.
// It returns up to limit results ordered by FTS5 rank (best match first).
func (d *Database) SearchInferences(query string, limit int) ([]Inference, error) {
	rows, err := d.db.Query(`
		SELECT i.id, i.context_type, i.subject, i.inference, i.evidence,
		       i.confidence, i.source, i.created_at, i.updated_at
		FROM inferences i
		JOIN inferences_fts fts ON i.id = fts.rowid
		WHERE inferences_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("SearchInferences: %w", err)
	}
	defer rows.Close()
	return scanInferences(rows)
}

// UpdateInferenceConfidence updates the confidence score and updated_at timestamp
// for the inference with the given ID.
func (d *Database) UpdateInferenceConfidence(id int64, newConf float64) error {
	_, err := d.db.Exec(`
		UPDATE inferences
		SET confidence = ?, updated_at = datetime('now')
		WHERE id = ?
	`, newConf, id)
	if err != nil {
		return fmt.Errorf("UpdateInferenceConfidence: %w", err)
	}
	return nil
}

// scanInferences scans all rows from an inferences query into a slice.
func scanInferences(rows *sql.Rows) ([]Inference, error) {
	var out []Inference
	for rows.Next() {
		var inf Inference
		var createdAt, updatedAt string
		if err := rows.Scan(
			&inf.ID, &inf.ContextType, &inf.Subject, &inf.Inference,
			&inf.Evidence, &inf.Confidence, &inf.Source,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanInferences: %w", err)
		}
		inf.CreatedAt = parseTimestamp(createdAt)
		inf.UpdatedAt = parseTimestamp(updatedAt)
		out = append(out, inf)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Correction CRUD
// ---------------------------------------------------------------------------

// InsertCorrection inserts a correction for an inference and returns the new row ID.
func (d *Database) InsertCorrection(c Correction) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO corrections (inference_id, correction, flagged_from, weight)
		VALUES (?, ?, ?, ?)
	`, c.InferenceID, c.Correction, c.FlaggedFrom, c.Weight)
	if err != nil {
		return 0, fmt.Errorf("InsertCorrection: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("InsertCorrection LastInsertId: %w", err)
	}
	return id, nil
}

// ListCorrectionsForInference returns all corrections for the given inference ID,
// ordered oldest-first.
func (d *Database) ListCorrectionsForInference(inferenceID int64) ([]Correction, error) {
	rows, err := d.db.Query(`
		SELECT id, inference_id, correction, flagged_from, weight, created_at
		FROM corrections
		WHERE inference_id = ?
		ORDER BY created_at ASC
	`, inferenceID)
	if err != nil {
		return nil, fmt.Errorf("ListCorrectionsForInference: %w", err)
	}
	defer rows.Close()

	var out []Correction
	for rows.Next() {
		var c Correction
		var createdAt string
		if err := rows.Scan(&c.ID, &c.InferenceID, &c.Correction, &c.FlaggedFrom, &c.Weight, &createdAt); err != nil {
			return nil, fmt.Errorf("ListCorrectionsForInference scan: %w", err)
		}
		c.CreatedAt = parseTimestamp(createdAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// ConfidenceThreshold CRUD
// ---------------------------------------------------------------------------

// GetOrCreateThreshold ensures a row exists for (actionType, workspace) and returns it.
// Uses INSERT OR IGNORE then SELECT — safe upsert without RETURNING.
func (d *Database) GetOrCreateThreshold(actionType, workspace string) (ConfidenceThreshold, error) {
	// Ensure the row exists with defaults.
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO confidence_thresholds (action_type, workspace)
		VALUES (?, ?)
	`, actionType, workspace)
	if err != nil {
		return ConfidenceThreshold{}, fmt.Errorf("GetOrCreateThreshold insert: %w", err)
	}

	// Retrieve the (possibly just-created) row.
	var ct ConfidenceThreshold
	var lastUpdated string
	err = d.db.QueryRow(`
		SELECT id, action_type, workspace, threshold, approvals, rejections, last_updated
		FROM confidence_thresholds
		WHERE action_type = ? AND workspace = ?
	`, actionType, workspace).Scan(
		&ct.ID, &ct.ActionType, &ct.Workspace,
		&ct.Threshold, &ct.Approvals, &ct.Rejections,
		&lastUpdated,
	)
	if err != nil {
		return ConfidenceThreshold{}, fmt.Errorf("GetOrCreateThreshold select: %w", err)
	}
	ct.LastUpdated = parseTimestamp(lastUpdated)
	return ct, nil
}

// RecordApproval increments the approval counter and recomputes the threshold.
// Formula: threshold = 0.70 + 0.20 * (approvals / (approvals + rejections)), capped at 0.95.
// Applied atomically in a single UPDATE statement.
func (d *Database) RecordApproval(actionType, workspace string) error {
	// Ensure row exists.
	if _, err := d.GetOrCreateThreshold(actionType, workspace); err != nil {
		return err
	}
	_, err := d.db.Exec(`
		UPDATE confidence_thresholds
		SET approvals    = approvals + 1,
		    threshold    = MIN(0.95, 0.70 + 0.20 * CAST(approvals + 1 AS REAL) / CAST(approvals + 1 + rejections AS REAL)),
		    last_updated = datetime('now')
		WHERE action_type = ? AND workspace = ?
	`, actionType, workspace)
	if err != nil {
		return fmt.Errorf("RecordApproval: %w", err)
	}
	return nil
}

// RecordRejection increments the rejection counter and recomputes the threshold.
// Formula: threshold = 0.70 + 0.20 * (approvals / (approvals + rejections)), capped at 0.95.
// Applied atomically in a single UPDATE statement.
func (d *Database) RecordRejection(actionType, workspace string) error {
	// Ensure row exists.
	if _, err := d.GetOrCreateThreshold(actionType, workspace); err != nil {
		return err
	}
	_, err := d.db.Exec(`
		UPDATE confidence_thresholds
		SET rejections   = rejections + 1,
		    threshold    = MIN(0.95, 0.70 + 0.20 * CAST(approvals AS REAL) / CAST(approvals + rejections + 1 AS REAL)),
		    last_updated = datetime('now')
		WHERE action_type = ? AND workspace = ?
	`, actionType, workspace)
	if err != nil {
		return fmt.Errorf("RecordRejection: %w", err)
	}
	return nil
}

// ListThresholds returns all confidence threshold rows.
func (d *Database) ListThresholds() ([]ConfidenceThreshold, error) {
	rows, err := d.db.Query(`
		SELECT id, action_type, workspace, threshold, approvals, rejections, last_updated
		FROM confidence_thresholds
		ORDER BY action_type ASC, workspace ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("ListThresholds: %w", err)
	}
	defer rows.Close()

	var out []ConfidenceThreshold
	for rows.Next() {
		var ct ConfidenceThreshold
		var lastUpdated string
		if err := rows.Scan(
			&ct.ID, &ct.ActionType, &ct.Workspace,
			&ct.Threshold, &ct.Approvals, &ct.Rejections,
			&lastUpdated,
		); err != nil {
			return nil, fmt.Errorf("ListThresholds scan: %w", err)
		}
		ct.LastUpdated = parseTimestamp(lastUpdated)
		out = append(out, ct)
	}
	return out, rows.Err()
}

// UpdateThreshold sets an explicit threshold value for (actionType, workspace).
// Used for manual overrides; does not modify the approvals/rejections counters.
func (d *Database) UpdateThreshold(actionType, workspace string, newThreshold float64) error {
	_, err := d.db.Exec(`
		UPDATE confidence_thresholds
		SET threshold    = ?,
		    last_updated = datetime('now')
		WHERE action_type = ? AND workspace = ?
	`, newThreshold, actionType, workspace)
	if err != nil {
		return fmt.Errorf("UpdateThreshold: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Skill CRUD
// ---------------------------------------------------------------------------

// Skill represents a promoted developer pattern — a recurring inference cluster
// that has crossed the emergence threshold without developer corrections.
type Skill struct {
	ID            int64
	Name          string
	Description   string
	ContextType   string
	EvidenceCount int
	PromotedAt    time.Time
	LastSeenAt    time.Time
}

// UpsertSkill inserts a new skill or updates an existing one by name.
// On conflict: updates description, evidence_count (taking the higher value),
// and last_seen_at; leaves promoted_at unchanged.
func (d *Database) UpsertSkill(name, description, contextType string, evidenceCount int) error {
	_, err := d.db.Exec(`
		INSERT INTO skills (name, description, context_type, evidence_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description    = excluded.description,
			evidence_count = MAX(evidence_count, excluded.evidence_count),
			last_seen_at   = datetime('now')
	`, name, description, contextType, evidenceCount)
	if err != nil {
		return fmt.Errorf("UpsertSkill: %w", err)
	}
	return nil
}

// ListSkills returns all skills ordered by promoted_at descending (newest first).
func (d *Database) ListSkills() ([]Skill, error) {
	rows, err := d.db.Query(`
		SELECT id, name, description, context_type, evidence_count, promoted_at, last_seen_at
		FROM skills
		ORDER BY promoted_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("ListSkills: %w", err)
	}
	defer rows.Close()

	var out []Skill
	for rows.Next() {
		var s Skill
		var promotedAt, lastSeenAt string
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.ContextType,
			&s.EvidenceCount, &promotedAt, &lastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("ListSkills scan: %w", err)
		}
		s.PromotedAt = parseTimestamp(promotedAt)
		s.LastSeenAt = parseTimestamp(lastSeenAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSkill retrieves a single skill by name.
// Returns nil, nil when no skill with that name exists.
func (d *Database) GetSkill(name string) (*Skill, error) {
	var s Skill
	var promotedAt, lastSeenAt string
	err := d.db.QueryRow(`
		SELECT id, name, description, context_type, evidence_count, promoted_at, last_seen_at
		FROM skills
		WHERE name = ?
	`, name).Scan(
		&s.ID, &s.Name, &s.Description, &s.ContextType,
		&s.EvidenceCount, &promotedAt, &lastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetSkill: %w", err)
	}
	s.PromotedAt = parseTimestamp(promotedAt)
	s.LastSeenAt = parseTimestamp(lastSeenAt)
	return &s, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// parseTimestamp parses a SQLite datetime string into time.Time.
// Tries RFC3339 first, then the SQLite default format.
func parseTimestamp(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
