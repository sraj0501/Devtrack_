package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func queueEvent(t *testing.T, d *Database, id, signature string) {
	t.Helper()
	_, err := d.InsertSageEvent(sage.Event{SchemaVersion: 1, EventID: id, Harness: "codex", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Now(), Command: signature, Signature: signature})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReferenceSecondVariantMergesWithoutCallingTheModel(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	w := knowledge.Writer{Root: t.TempDir()}
	draft := distill.Draft{Title: "Status", Section: "Git", Commands: []string{"git status --short"}, What: "Shows changes", Why: "Review", Example: "Before commit"}
	if _, err := w.Write("git", "abcdef21", draft); err != nil {
		t.Fatal(err)
	}
	queueEvent(t, database, "variant", "git status --porcelain=v2")
	q := SageQueue{Database: database, Now: func() time.Time { return time.Unix(100, 0) }}
	reused, err := q.ReuseDocumented(context.Background(), w)
	if err != nil || !reused {
		t.Fatalf("reused=%v err=%v", reused, err)
	}
	if _, found, err := q.Claim(context.Background()); err != nil || found {
		t.Fatalf("variant reached model queue: found=%v err=%v", found, err)
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, "git.md"))
	if err != nil || len(knowledge.ParseMarkdown(string(raw))[0].Signatures) != 2 {
		t.Fatalf("variant not attached: %s err=%v", raw, err)
	}
	var state, filename string
	if err := database.DB().QueryRow(`SELECT j.state,p.filename FROM sage_jobs j JOIN sage_publications p USING(signature)`).Scan(&state, &filename); err != nil || state != "distilled" || filename != "git.md" {
		t.Fatalf("state=%s filename=%s err=%v", state, filename, err)
	}
}

func TestReferenceAlreadyDocumentedSigIsUntouched(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	w := knowledge.Writer{Root: t.TempDir()}
	signature := "git status --short"
	digest := sha256.Sum256([]byte(signature))
	hashed := hex.EncodeToString(digest[:])
	draft := distill.Draft{Title: "Status", Section: "Git", Commands: []string{signature}, What: "Shows changes", Why: "Review", Example: "Before commit"}
	if _, err := w.Write("git", hashed, draft); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(w.Root, "git.md"))
	queueEvent(t, database, "same", signature)
	q := SageQueue{Database: database, Now: func() time.Time { return time.Unix(100, 0) }}
	if reused, err := q.ReuseDocumented(context.Background(), w); err != nil || !reused {
		t.Fatalf("reused=%v err=%v", reused, err)
	}
	after, _ := os.ReadFile(filepath.Join(w.Root, "git.md"))
	if string(before) != string(after) {
		t.Fatal("already documented entry was rewritten")
	}
}

func TestSageQueueRestartRetriesAndFencesStaleClaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	queueEvent(t, d, "one", "git status")
	now := time.Unix(1000, 0)
	q := &SageQueue{Database: d, Lease: time.Minute, RetryDelay: time.Second, Now: func() time.Time { return now }}
	ctx := context.Background()
	first, found, err := q.Claim(ctx)
	if err != nil || !found {
		t.Fatalf("claim: %v %v", found, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	q.Database = d
	if _, found, err := q.Claim(ctx); err != nil || found {
		t.Fatalf("live lease stolen: %v %v", found, err)
	}
	now = now.Add(time.Minute)
	second, found, err := q.Claim(ctx)
	if err != nil || !found || second.ID == first.ID {
		t.Fatalf("reclaim: %+v %v %v", second, found, err)
	}
	if err := q.Complete(ctx, first.ID, distill.Draft{Title: "stale"}); !errors.Is(err, ErrSageClaimLost) {
		t.Fatalf("stale completion: %v", err)
	}
	if err := q.Retry(ctx, second.ID, errors.New("SECRET_CANARY https://private/token")); err != nil {
		t.Fatal(err)
	}
	var diagnostic string
	if err := d.db.QueryRow(`SELECT last_error FROM sage_jobs`).Scan(&diagnostic); err != nil || strings.Contains(diagnostic, "CANARY") {
		t.Fatalf("diagnostic=%q err=%v", diagnostic, err)
	}
	if _, found, err := q.Claim(ctx); err != nil || found {
		t.Fatalf("early retry: %v %v", found, err)
	}
	now = now.Add(time.Second)
	third, found, err := q.Claim(ctx)
	if err != nil || !found {
		t.Fatalf("retry claim: %v %v", found, err)
	}
	if err := q.Complete(ctx, third.ID, distill.Draft{Title: "Saved draft"}); err != nil {
		t.Fatal(err)
	}
	queueEvent(t, d, "variant", "git status")
	if _, found, err := q.Claim(ctx); err != nil || found {
		t.Fatalf("completed signature repeated: %v %v", found, err)
	}
	var state, raw string
	var attempts int
	if err := d.db.QueryRow(`SELECT state,draft_json,attempts FROM sage_jobs`).Scan(&state, &raw, &attempts); err != nil || state != "distilled" || attempts != 3 || !strings.Contains(raw, "Saved draft") {
		t.Fatalf("state=%s attempts=%d raw=%s err=%v", state, attempts, raw, err)
	}
}

func TestSageQueueConcurrentClaimsAndPersistentSkip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	other, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	queueEvent(t, d, "one", "git status")
	ctx := context.Background()
	jobs := make(chan distill.Job, 2)
	var wg sync.WaitGroup
	for _, database := range []*Database{d, other} {
		wg.Add(1)
		go func(database *Database) {
			defer wg.Done()
			q := &SageQueue{Database: database}
			job, found, err := q.Claim(ctx)
			if err != nil {
				t.Error(err)
			}
			if found {
				jobs <- job
			}
		}(database)
	}
	wg.Wait()
	close(jobs)
	if len(jobs) != 1 {
		t.Fatalf("claimed %d times", len(jobs))
	}
	job := <-jobs
	q := &SageQueue{Database: d}
	if err := q.Skip(ctx, job.ID, "explicit model verdict SECRET_CANARY"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := q.Claim(ctx); err != nil || found {
		t.Fatalf("skip repeated: %v %v", found, err)
	}
	var state, reason string
	if err := other.db.QueryRow(`SELECT state,skip_reason FROM sage_jobs`).Scan(&state, &reason); err != nil || state != "skipped" || reason == "" || strings.Contains(reason, "CANARY") {
		t.Fatalf("state=%s reason=%s err=%v", state, reason, err)
	}
}

func TestSageQueueMigrationBackfillAndImportAtomicity(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.db.Exec(`DROP TRIGGER sage_jobs_enqueue; DROP TABLE sage_jobs`); err != nil {
		t.Fatal(err)
	}
	queueEvent(t, d, "legacy", "git status")
	for i := 0; i < 2; i++ {
		if err := d.createSageQueue(); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM sage_jobs`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backfill count=%d err=%v", count, err)
	}
	if _, err := d.db.Exec(`CREATE TRIGGER reject_sage_job BEFORE INSERT ON sage_jobs BEGIN SELECT RAISE(ABORT,'disk fault'); END`); err != nil {
		t.Fatal(err)
	}
	event := sage.Event{SchemaVersion: 1, EventID: "new", Harness: "codex", SessionID: "s", EventType: "command", Tool: "shell", OccurredAt: time.Now(), Command: "go test", Signature: "go test"}
	if _, err := d.InsertSageEvent(event); err == nil {
		t.Fatal("injected enqueue failure succeeded")
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM sage_events WHERE signature='go test'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("event escaped rollback: %d %v", count, err)
	}
	if _, err := d.db.Exec(`DROP TRIGGER reject_sage_job`); err != nil {
		t.Fatal(err)
	}
	if inserted, err := d.InsertSageEvent(event); err != nil || !inserted {
		t.Fatalf("event cannot retry: %v %v", inserted, err)
	}
}
