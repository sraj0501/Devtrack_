package db

import (
	"path/filepath"
	"strings"
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

func TestSageKnowledgeGroupsSearchesAndAttributesEvents(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	success, failure := true, false
	events := []sage.Event{
		{SchemaVersion: 1, EventID: "first", Harness: "codex", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(20, 0).UTC(), Command: "git status --short", Signature: "git status", Success: &success},
		{SchemaVersion: 1, EventID: "second", Harness: "codex-history", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git status", Signature: "git status", Success: &failure},
		{SchemaVersion: 1, EventID: "third", Harness: "codex", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(30, 0).UTC(), Command: "docker ps", Signature: "docker ps", Success: &success},
	}
	for _, event := range events {
		inserted, err := database.InsertSageEvent(event)
		if err != nil || !inserted {
			t.Fatalf("insert %s: inserted=%v err=%v", event.EventID, inserted, err)
		}
	}
	inserted, err := database.InsertSageEvent(events[0])
	if err != nil || inserted {
		t.Fatalf("duplicate: inserted=%v err=%v", inserted, err)
	}

	results, err := database.SearchSageKnowledge("git status", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%+v", results)
	}
	entry := results[0]
	if entry.Signature != "git status" || entry.Topic != "git" || entry.Command != "git status" ||
		entry.UseCount != 2 || entry.SuccessCount != 1 || entry.FailureCount != 1 ||
		strings.Join(entry.SourceHarnesses, ",") != "codex,codex-history" {
		t.Fatalf("entry=%+v", entry)
	}
	if !entry.FirstSeen.Equal(time.Unix(10, 0).UTC()) || !entry.LastSeen.Equal(time.Unix(20, 0).UTC()) {
		t.Fatalf("time range=%s..%s", entry.FirstSeen, entry.LastSeen)
	}

	topics, err := database.ListSageTopics()
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 || topics[0].Name != "docker" || topics[1].Name != "git" || topics[1].Uses != 2 {
		t.Fatalf("topics=%+v", topics)
	}
}

func TestSageKnowledgeSearchTreatsFTSSyntaxAsLiteralTerms(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	success := true
	event := sage.Event{SchemaVersion: 1, EventID: "event", Harness: "codex", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git status", Signature: "git status", Success: &success}
	if _, err := database.InsertSageEvent(event); err != nil {
		t.Fatal(err)
	}
	results, err := database.SearchSageKnowledge(`git OR topic:secret`, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("operator-like query should be literal: %+v", results)
	}
}

func TestSageKnowledgeBackfillsExistingEventsOnce(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	success := true
	event := sage.Event{SchemaVersion: 1, EventID: "legacy", Harness: "codex", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git log --oneline", Signature: "git log", Success: &success}
	if _, err := database.DB().Exec(`
		INSERT INTO sage_events (
			delivery_key, schema_version, event_id, harness, session_id, event_type,
			tool, occurred_at, project_id, command, signature, success
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.DeliveryKey(), event.SchemaVersion, event.EventID, event.Harness, event.SessionID,
		event.EventType, event.Tool, event.OccurredAt, event.ProjectID, event.Command, event.Signature, event.Success); err != nil {
		t.Fatal(err)
	}
	if err := database.createSageKnowledgeTables(); err != nil {
		t.Fatal(err)
	}
	if err := database.createSageKnowledgeTables(); err != nil {
		t.Fatal(err)
	}
	results, err := database.SearchSageKnowledge("git log", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].UseCount != 1 || results[0].Signature != "git log" {
		t.Fatalf("results=%+v", results)
	}
}
