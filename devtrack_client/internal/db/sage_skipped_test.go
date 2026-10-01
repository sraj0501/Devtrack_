package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

type cancellingSkipPublisher struct {
	knowledge.Writer
	cancel context.CancelFunc
}

func (w cancellingSkipPublisher) WriteSkipped(sig string) (string, error) {
	name, err := w.Writer.WriteSkipped(sig)
	w.cancel()
	return name, err
}

func TestReferenceSkipVerdictIsRecordedSoItNeverReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.Close() }()
	ctx := context.Background()
	queueEvent(t, d, "skip-one", "git status")
	now := time.Unix(1000, 0)
	q := &SageQueue{Database: d, Now: func() time.Time { return now }, RetryDelay: time.Second}
	job, found, err := q.Claim(ctx)
	if err != nil || !found {
		t.Fatalf("claim %v %v", found, err)
	}
	if err := q.Skip(ctx, job.ID, "SECRET_CANARY model prose"); err != nil {
		t.Fatal(err)
	}
	w := knowledge.Writer{Root: t.TempDir()}
	blocked := filepath.Join(w.Root, "_skipped.md")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if found, err := q.PublishNext(ctx, w); !found || err == nil {
		t.Fatalf("expected retry: %v %v", found, err)
	}
	if err := os.Remove(blocked); err != nil {
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
	cancelCtx, cancel := context.WithCancel(ctx)
	if _, err := q.PublishNext(cancelCtx, cancellingSkipPublisher{w, cancel}); err == nil {
		t.Fatal("acknowledged cancelled publication")
	}
	before, err := os.ReadFile(blocked)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.PublishNext(ctx, w); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(blocked)
	if err != nil || string(before) != string(after) || len(knowledge.ParseMarkdown(string(after))) != 1 || strings.Contains(string(after), "CANARY") {
		t.Fatalf("invalid replay/privacy: %v", err)
	}
	queueEvent(t, d, "skip-two", "git status")
	if _, found, err := q.Claim(ctx); found || err != nil {
		t.Fatalf("repeated model: %v %v", found, err)
	}
	var state, filename string
	if err := d.DB().QueryRow(`SELECT state,filename FROM sage_publications`).Scan(&state, &filename); err != nil || state != "documented" || filename != "_skipped.md" {
		t.Fatalf("%s %s %v", state, filename, err)
	}
	// Migration repairs skips made by the previous schema without duplicating records.
	if _, err := d.DB().Exec(`DELETE FROM sage_publications`); err != nil {
		t.Fatal(err)
	}
	if err := d.createSageQueue(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.PublishNext(ctx, w); err != nil {
		t.Fatal(err)
	}
	final, _ := os.ReadFile(blocked)
	if string(final) != string(after) {
		t.Fatal("backfill duplicated skip")
	}
}

func TestReferenceModelOutageIsNotRecordedAsSkipped(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	queueEvent(t, d, "outage", "git status")
	q := &SageQueue{Database: d}
	job, _, err := q.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Retry(context.Background(), job.ID, errors.New("SECRET_CANARY")); err != nil {
		t.Fatal(err)
	}
	w := knowledge.Writer{Root: t.TempDir()}
	if found, err := q.PublishNext(context.Background(), w); found || err != nil {
		t.Fatalf("outage published: %v %v", found, err)
	}
	files, err := os.ReadDir(w.Root)
	if err != nil || len(files) != 0 {
		t.Fatalf("outage wrote files: %v", err)
	}
}
