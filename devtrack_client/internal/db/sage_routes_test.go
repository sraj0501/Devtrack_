package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func TestSageRouteCorrectionSurvivesRestartAndPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.Close() }()
	w := knowledge.Writer{Root: t.TempDir()}
	ctx := context.Background()
	if err := d.RememberSageRoute(ctx, w, "git", "version-control"); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	q := &SageQueue{Database: d}
	for i, command := range []string{"git status", "git log"} {
		queueEvent(t, d, command, command)
		job, found, err := q.Claim(ctx)
		if err != nil || !found {
			t.Fatalf("claim: %v %v", found, err)
		}
		draft := distill.Draft{Title: command, Commands: []string{command}, What: "Changes", Why: "Review", Example: command}
		if err := q.Complete(ctx, job.ID, draft); err != nil {
			t.Fatal(err)
		}
		if found, err := q.PublishNext(ctx, w); err != nil || !found {
			t.Fatalf("publish: %v %v", found, err)
		}
		var name string
		if err := d.db.QueryRow(`SELECT filename FROM sage_publications WHERE signature=?`, command).Scan(&name); err != nil || name != "version-control.md" {
			t.Fatalf("%s %v", name, err)
		}
		if i == 0 { // An old/new classifier decision cannot override the documented correction.
			if err := d.RememberSageRoute(ctx, w, "git", "wrong"); err != nil {
				t.Fatal(err)
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, "version-control.md"))
	if err != nil || len(knowledge.ParseMarkdown(string(raw))) != 2 {
		t.Fatalf("%s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "wrong.md")); !os.IsNotExist(err) {
		t.Fatalf("stale route published: %v", err)
	}
}

func TestSageRouteCorrectionWaitsForPublicationLock(t *testing.T) {
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
	ctx := context.Background()
	job, _, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(ctx, job.ID, distill.Draft{}); err != nil {
		t.Fatal(err)
	}
	publisher := &serialPublisher{entered: make(chan struct{}), release: make(chan struct{})}
	published := make(chan error, 1)
	go func() { _, err := q.PublishNext(ctx, publisher); published <- err }()
	select {
	case <-publisher.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("publisher blocked")
	}
	w := knowledge.Writer{Root: t.TempDir()}
	done := make(chan error, 1)
	go func() { done <- other.RememberSageRoute(ctx, w, "git", "version-control") }()
	select {
	case err := <-done:
		close(publisher.release)
		t.Fatalf("route bypassed publication lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(publisher.release)
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSageMergeThenRestartAndPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.Close() }()
	w := knowledge.Writer{Root: t.TempDir()}
	ctx := context.Background()
	initial := distill.Draft{Title: "Grep", Commands: []string{"grep foo"}, What: "Find", Why: "Inspect", Example: "grep foo"}
	if _, err := w.Write("grep", "aaaaaaaaaaaa", initial); err != nil {
		t.Fatal(err)
	}
	if n, err := d.MergeSageTopics(ctx, w, "grep", "shell"); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	queueEvent(t, d, "grep bar", "grep bar")
	q := &SageQueue{Database: d}
	job, found, err := q.Claim(ctx)
	if err != nil || !found {
		t.Fatalf("claim: %v %v", found, err)
	}
	initial.Commands = []string{"grep bar"}
	if err := q.Complete(ctx, job.ID, initial); err != nil {
		t.Fatal(err)
	}
	if found, err := q.PublishNext(ctx, w); err != nil || !found {
		t.Fatalf("publish: %v %v", found, err)
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, "shell.md"))
	if err != nil || len(knowledge.ParseMarkdown(string(raw))) != 2 {
		t.Fatalf("%s %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "grep.md")); !os.IsNotExist(err) {
		t.Fatal("old topic recreated", err)
	}
}
