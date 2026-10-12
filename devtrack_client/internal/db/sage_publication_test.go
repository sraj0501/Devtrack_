package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

type serialPublisher struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (w *serialPublisher) Write(_, _ string, _ distill.Draft) (string, error) {
	if w.calls.Add(1) == 1 {
		close(w.entered)
		<-w.release
	}
	return "git.md", nil
}

func TestSagePublicationCancellationRetainsLockUntilWriterReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
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
	q := &SageQueue{Database: d}
	job, _, err := q.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(context.Background(), job.ID, distill.Draft{}); err != nil {
		t.Fatal(err)
	}
	w := &serialPublisher{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() { _, err := q.PublishNext(ctx, w); first <- err }()
	select {
	case <-w.entered:
	case err := <-first:
		t.Fatalf("first writer failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("writer did not enter")
	}
	cancel()
	second := make(chan error, 1)
	go func() { _, err := (&SageQueue{Database: other}).PublishNext(context.Background(), w); second <- err }()
	time.Sleep(100 * time.Millisecond)
	calls := w.calls.Load()
	close(w.release)
	if err := <-first; err == nil {
		t.Error("cancelled publication acknowledged")
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if calls != 1 || w.calls.Load() != 2 {
		t.Fatalf("concurrent writer entered before lock release: before=%d after=%d", calls, w.calls.Load())
	}
}

type cancellingPublisher struct {
	writer knowledge.Writer
	cancel context.CancelFunc
}

func (w cancellingPublisher) Write(topic, signature string, draft distill.Draft) (string, error) {
	name, err := w.writer.Write(topic, signature, draft)
	w.cancel()
	return name, err
}

func TestSagePublicationFailureRestartAndAcknowledgementRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.Close() }()
	queueEvent(t, d, "one", "git status")
	now := time.Unix(1000, 0)
	q := &SageQueue{Database: d, Now: func() time.Time { return now }, RetryDelay: time.Second}
	ctx := context.Background()
	job, found, err := q.Claim(ctx)
	if err != nil || !found {
		t.Fatalf("claim: %v %v", found, err)
	}
	draft := distill.Draft{Title: "Status", Section: "Git", Commands: []string{"git status"}, What: "Shows changes", Why: "Review work", Example: "git status"}
	if err := q.Complete(ctx, job.ID, draft); err != nil {
		t.Fatal(err)
	}
	w := knowledge.Writer{Root: t.TempDir()}
	index := filepath.Join(w.Root, "README.md")
	// Topic succeeds but malformed user index refuses replacement.
	if err := os.WriteFile(index, []byte("SECRET_CANARY\n<!-- sage:topics:start -->"), 0600); err != nil {
		t.Fatal(err)
	}
	if found, err := q.PublishNext(ctx, w); !found || err == nil {
		t.Fatalf("failure: %v %v", found, err)
	}
	var state, diagnostic string
	if err := d.DB().QueryRow(`SELECT state,last_error FROM sage_publications`).Scan(&state, &diagnostic); err != nil || state != "retrying" || strings.Contains(diagnostic, "CANARY") {
		t.Fatalf("%s %s %v", state, diagnostic, err)
	}
	if found, err := q.PublishNext(ctx, w); found || err != nil {
		t.Fatalf("early retry: %v %v", found, err)
	}
	before, err := os.ReadFile(filepath.Join(w.Root, "git.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	q.Database = d
	now = now.Add(time.Second)
	if err := os.WriteFile(index, []byte("# My notes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Both files now succeed but shutdown prevents SQLite acknowledgement.
	cancelCtx, cancel := context.WithCancel(ctx)
	if found, err := q.PublishNext(cancelCtx, cancellingPublisher{w, cancel}); !found || err == nil {
		t.Fatalf("cancel: %v %v", found, err)
	}
	if found, err := q.PublishNext(ctx, w); !found || err != nil {
		t.Fatalf("replay: %v %v", found, err)
	}
	after, err := os.ReadFile(filepath.Join(w.Root, "git.md"))
	if err != nil || string(before) != string(after) || len(knowledge.ParseMarkdown(string(after))) != 1 {
		t.Fatalf("topic changed on recovery: %v", err)
	}
	var attempts int
	var filename string
	if err := d.DB().QueryRow(`SELECT state,attempts,filename FROM sage_publications`).Scan(&state, &attempts, &filename); err != nil || state != "documented" || attempts != 2 || filename != "git.md" {
		t.Fatalf("%s %d %s %v", state, attempts, filename, err)
	}
	if found, err := q.PublishNext(ctx, w); found || err != nil {
		t.Fatalf("duplicate: %v %v", found, err)
	}
	if _, found, err := q.Claim(ctx); found || err != nil {
		t.Fatalf("model repeated: %v %v", found, err)
	}
}

func TestSagePublicationBackfillsExistingDrafts(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	queueEvent(t, d, "one", "git status")
	if _, err := d.DB().Exec(`DROP TRIGGER sage_publications_enqueue; DROP TABLE sage_publications;
		UPDATE sage_jobs SET state='distilled',draft_json='{}'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := d.createSageQueue(); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := d.DB().QueryRow(`SELECT count(*) FROM sage_publications`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backfill %d %v", count, err)
	}
}
