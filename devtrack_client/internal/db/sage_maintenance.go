package db

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SageRetentionResult describes one bounded raw-event retention pass. Durable
// knowledge, jobs, publications, and Markdown are deliberately not deleted.
type SageRetentionResult struct {
	Before     time.Time `json:"before"`
	Candidates int       `json:"candidates"`
	Removed    int       `json:"removed"`
	Applied    bool      `json:"applied"`
}

// RetainSageEvents previews or applies deletion of normalized source events
// older than before. Source links are removed first so summarized knowledge is
// preserved without dangling foreign keys.
func (d *Database) RetainSageEvents(ctx context.Context, before time.Time, apply bool) (SageRetentionResult, error) {
	result := SageRetentionResult{Before: before.UTC(), Applied: apply}
	if before.IsZero() {
		return result, errors.New("Sage retention cutoff is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin Sage retention: %w", err)
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sage_events WHERE occurred_at < ?`, result.Before).Scan(&result.Candidates); err != nil {
		return result, fmt.Errorf("preview Sage retention: %w", err)
	}
	if !apply {
		return result, tx.Rollback()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sage_knowledge_sources
		WHERE delivery_key IN (SELECT delivery_key FROM sage_events WHERE occurred_at < ?)`, result.Before); err != nil {
		return result, fmt.Errorf("unlink retained Sage events: %w", err)
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM sage_events WHERE occurred_at < ?`, result.Before)
	if err != nil {
		return result, fmt.Errorf("delete retained Sage events: %w", err)
	}
	rows, err := deleted.RowsAffected()
	if err != nil {
		return result, fmt.Errorf("count retained Sage events: %w", err)
	}
	result.Removed = int(rows)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit Sage retention: %w", err)
	}
	return result, nil
}

// WithSageMaintenance serializes a local filesystem maintenance operation with
// publication and merge transactions in other DevTrack processes. Callers must
// also require paused capture before invoking it.
func (d *Database) WithSageMaintenance(ctx context.Context, operation func() error) error {
	if operation == nil {
		return errors.New("Sage maintenance operation is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	maintenanceCtx := context.WithoutCancel(ctx)
	tx, err := d.db.BeginTx(maintenanceCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(maintenanceCtx, `UPDATE sage_publications SET attempts=attempts WHERE 0`); err != nil {
		return err
	}
	if err := operation(); err != nil {
		return err
	}
	return tx.Commit()
}
