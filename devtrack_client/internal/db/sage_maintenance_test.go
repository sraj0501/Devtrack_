package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestSageRetentionDryRunApplyAndKnowledgePreservation(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	old := sage.Event{SchemaVersion: sage.SchemaVersion, EventID: "old", Harness: "codex", SessionID: "s", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git status", Signature: "git status"}
	recent := old
	recent.EventID, recent.OccurredAt = "recent", time.Unix(30, 0).UTC()
	for _, event := range []sage.Event{old, recent} {
		if inserted, err := d.InsertSageEvent(event); err != nil || !inserted {
			t.Fatalf("insert=%t err=%v", inserted, err)
		}
	}
	ctx := context.Background()
	preview, err := d.RetainSageEvents(ctx, time.Unix(20, 0), false)
	if err != nil || preview.Candidates != 1 || preview.Removed != 0 || preview.Applied {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	var events int
	if err := d.DB().QueryRow(`SELECT COUNT(*) FROM sage_events`).Scan(&events); err != nil || events != 2 {
		t.Fatalf("dry run changed events: %d %v", events, err)
	}
	applied, err := d.RetainSageEvents(ctx, time.Unix(20, 0), true)
	if err != nil || applied.Removed != 1 || !applied.Applied {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	var knowledge, sources int
	if err := d.DB().QueryRow(`SELECT COUNT(*) FROM sage_knowledge`).Scan(&knowledge); err != nil || knowledge != 1 {
		t.Fatalf("knowledge=%d err=%v", knowledge, err)
	}
	if err := d.DB().QueryRow(`SELECT COUNT(*) FROM sage_knowledge_sources`).Scan(&sources); err != nil || sources != 1 {
		t.Fatalf("sources=%d err=%v", sources, err)
	}
	second, err := d.RetainSageEvents(ctx, time.Unix(20, 0), true)
	if err != nil || second.Candidates != 0 || second.Removed != 0 {
		t.Fatalf("retry=%+v err=%v", second, err)
	}
}

func TestSageMaintenanceCancellationAndFailureRollback(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := d.WithSageMaintenance(ctx, func() error { called = true; return nil }); !errors.Is(err, context.Canceled) || called {
		t.Fatalf("cancel err=%v called=%t", err, called)
	}
	canary := errors.New("maintenance failed")
	if err := d.WithSageMaintenance(context.Background(), func() error { return canary }); !errors.Is(err, canary) {
		t.Fatalf("operation error=%v", err)
	}
}
