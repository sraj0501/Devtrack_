package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestInsertSageEventIsAppendOnlyAndDeduplicated(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	success, code := true, 0
	event := sage.Event{SchemaVersion: 1, EventID: "event", Harness: "codex-history", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git status", Signature: "git status", Success: &success, ExitCode: &code}
	inserted, err := database.InsertSageEvent(event)
	if err != nil || !inserted {
		t.Fatalf("first insert: %v, %v", inserted, err)
	}
	inserted, err = database.InsertSageEvent(event)
	if err != nil || inserted {
		t.Fatalf("duplicate insert: %v, %v", inserted, err)
	}
	var count int
	if err := database.DB().QueryRow(`SELECT COUNT(*) FROM sage_events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
