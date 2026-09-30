package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

// SageQueue persists model work independently from capture and Git operations.
// IDs returned to workers are fencing tokens, not stable event identifiers.
type SageQueue struct {
	Database   *Database
	Lease      time.Duration
	RetryDelay time.Duration
	Now        func() time.Time
}

var ErrSageClaimLost = errors.New("sage processing lease expired or replaced")

func (d *Database) createSageQueue() error {
	tx, err := d.beginSchemaTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS sage_jobs (
		signature TEXT PRIMARY KEY,
		command TEXT NOT NULL,
		failed INTEGER NOT NULL,
		state TEXT NOT NULL DEFAULT 'waiting' CHECK(state IN ('waiting','processing','retrying','distilled','skipped')),
		attempts INTEGER NOT NULL DEFAULT 0,
		next_retry_ms INTEGER NOT NULL DEFAULT 0,
		lease_until_ms INTEGER NOT NULL DEFAULT 0,
		claim_token TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		skip_reason TEXT NOT NULL DEFAULT '',
		draft_json TEXT NOT NULL DEFAULT '',
		completed_ms INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_sage_jobs_due ON sage_jobs(state,next_retry_ms,lease_until_ms);
	CREATE TRIGGER IF NOT EXISTS sage_jobs_enqueue AFTER INSERT ON sage_knowledge BEGIN
		INSERT OR IGNORE INTO sage_jobs(signature,command,failed)
		VALUES (new.signature,new.representative_command,
		 CASE WHEN new.success_count=0 AND new.failure_count>0 THEN 1 ELSE 0 END);
	END;
	INSERT OR IGNORE INTO sage_jobs(signature,command,failed)
		SELECT signature,representative_command,CASE WHEN success_count=0 AND failure_count>0 THEN 1 ELSE 0 END
		FROM sage_knowledge;`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (q *SageQueue) now() time.Time {
	if q.Now != nil {
		return q.Now()
	}
	return time.Now()
}

func (q *SageQueue) Claim(ctx context.Context) (distill.Job, bool, error) {
	// Import enqueues transactionally through a trigger. Idle polling does not
	// rescan command history; migration backfills pre-existing signatures once.
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return distill.Job{}, false, err
	}
	lease := q.Lease
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	now := q.now()
	job := distill.Job{ID: hex.EncodeToString(token[:])}
	err := q.Database.db.QueryRowContext(ctx, `UPDATE sage_jobs SET
		state='processing',attempts=attempts+1,claim_token=?,lease_until_ms=?
		WHERE signature=(SELECT signature FROM sage_jobs
		 WHERE (state IN ('waiting','retrying') AND next_retry_ms<=?)
		 OR (state='processing' AND lease_until_ms<=?)
		 ORDER BY next_retry_ms,signature LIMIT 1)
		RETURNING command,failed`, job.ID, now.Add(lease).UnixMilli(), now.UnixMilli(), now.UnixMilli()).Scan(&job.Fact.Command, &job.Fact.Failed)
	if errors.Is(err, sql.ErrNoRows) {
		return distill.Job{}, false, nil
	}
	return job, err == nil, err
}

func (q *SageQueue) finish(ctx context.Context, token, state, draft, reason, diagnostic string, retryAt int64) error {
	now := q.now().UnixMilli()
	completed := now
	if state == "retrying" {
		completed = 0
	}
	result, err := q.Database.db.ExecContext(ctx, `UPDATE sage_jobs SET state=?,draft_json=?,skip_reason=?,last_error=?,
		next_retry_ms=?,completed_ms=?,claim_token='',lease_until_ms=0
		WHERE claim_token=? AND state='processing' AND lease_until_ms>?`,
		state, draft, reason, diagnostic, retryAt, completed, token, now)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrSageClaimLost
	}
	return nil
}

func (q *SageQueue) Complete(ctx context.Context, token string, draft distill.Draft) error {
	raw, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	return q.finish(ctx, token, "distilled", string(raw), "", "", 0)
}

func (q *SageQueue) Skip(ctx context.Context, token, reason string) error {
	if reason == "" {
		return errors.New("sage skip requires an explicit reason")
	}
	// Model prose can echo sensitive input. Keep a stable local verdict category.
	return q.finish(ctx, token, "skipped", "", "model judged command not useful", "", 0)
}

func (q *SageQueue) Retry(ctx context.Context, token string, cause error) error {
	delay := q.RetryDelay
	if delay <= 0 {
		delay = 2 * time.Second
	}
	// Never persist raw HTTP response bodies, URLs, credentials, or model prose.
	diagnostic := "model or persistence failure"
	if errors.Is(cause, context.DeadlineExceeded) {
		diagnostic = "model timeout"
	}
	return q.finish(ctx, token, "retrying", "", "", diagnostic, q.now().Add(delay).UnixMilli())
}
