package db

import (
	"context"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

// RememberSageRoute serializes CLI corrections with publication and other CLI
// processes using the same database. Cancellation cannot release the lock
// while an atomic file replacement is still running.
func (d *Database) RememberSageRoute(ctx context.Context, writer knowledge.Writer, binary, topic string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE sage_publications SET attempts=attempts WHERE 0`); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// No SQL acknowledgement is needed: the atomic routes file is authoritative.
	if err := writer.RememberRoute(binary, topic); err != nil {
		return err
	}
	return tx.Commit()
}
