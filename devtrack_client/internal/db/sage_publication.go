package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

// SagePublisher must perform local, idempotent file operations only. No model,
// network, or Git work is permitted while the publication transaction is held.
type SagePublisher interface {
	Write(topic, signature string, draft distill.Draft) (string, error)
}

// PublishNext serializes publication through SQLite's writer lock. A process
// crash rolls back the claim; replay repairs a topic/index written before the
// acknowledgement. Distillation is independent and never repeated on file errors.
func (q *SageQueue) PublishNext(ctx context.Context, writer SagePublisher) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// File replacement cannot be interrupted. Keep the lock until it returns,
	// even if cancellation arrives, rather than letting database/sql roll back
	// automatically while a different process begins writing the same files.
	tx, err := q.Database.db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var signature string
	err = tx.QueryRowContext(ctx, `UPDATE sage_publications SET attempts=attempts+1
		WHERE signature=(SELECT signature FROM sage_publications
		WHERE state IN ('waiting','retrying') AND next_retry_ms<=?
		ORDER BY next_retry_ms,signature LIMIT 1) RETURNING signature`, q.now().UnixMilli()).Scan(&signature)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var raw, topic string
	err = tx.QueryRowContext(ctx, `SELECT j.draft_json,k.topic FROM sage_jobs j
		JOIN sage_knowledge k ON k.signature=j.signature WHERE j.signature=? AND j.state='distilled'`, signature).Scan(&raw, &topic)
	if err != nil {
		return true, err
	}
	var draft distill.Draft
	err = json.Unmarshal([]byte(raw), &draft)
	var filename string
	if err == nil && ctx.Err() == nil {
		// Hash the canonical normalized signature, never the random claim token.
		digest := sha256.Sum256([]byte(signature))
		filename, err = writer.Write(topic, hex.EncodeToString(digest[:]), draft)
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	if err != nil {
		delay := q.RetryDelay
		if delay <= 0 {
			delay = 2 * time.Second
		}
		_, updateErr := tx.ExecContext(ctx, `UPDATE sage_publications SET state='retrying',
			next_retry_ms=?,last_error='knowledge publication failure' WHERE signature=?`, q.now().Add(delay).UnixMilli(), signature)
		if updateErr != nil {
			return true, updateErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return true, commitErr
		}
		return true, errors.New("sage knowledge publication failed")
	}
	_, err = tx.ExecContext(ctx, `UPDATE sage_publications SET state='documented',filename=?,
		completed_ms=?,next_retry_ms=0,last_error='' WHERE signature=?`, filename, q.now().UnixMilli(), signature)
	if err != nil {
		return true, err
	}
	return true, tx.Commit()
}
