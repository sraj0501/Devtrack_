package db

import (
	"testing"
	"time"
)

// TestInsertTrigger_TicketIDRoundTrip confirms TASK-068 acceptance criterion:
// TriggerRecord.TicketID is persisted and read back for every commit trigger.
func TestInsertTrigger_TicketIDRoundTrip(t *testing.T) {
	database := newTestDB(t)

	id, err := database.InsertTrigger(TriggerRecord{
		TriggerType:   "commit",
		Timestamp:     time.Now(),
		Source:        "git",
		RepoPath:      "/repo/devtrack",
		CommitHash:    "abc1234",
		CommitMessage: "feat: add login",
		Author:        "dev",
		TicketID:      "PROJ-123",
	})
	if err != nil {
		t.Fatalf("InsertTrigger: %v", err)
	}

	got, err := database.GetTriggerByID(id)
	if err != nil {
		t.Fatalf("GetTriggerByID: %v", err)
	}
	if got.TicketID != "PROJ-123" {
		t.Errorf("TicketID = %q, want %q", got.TicketID, "PROJ-123")
	}
}

// TestInsertTrigger_UnlinkedTicketID confirms a commit with no extractable
// ticket ID stores an empty string ("unlinked") without error.
func TestInsertTrigger_UnlinkedTicketID(t *testing.T) {
	database := newTestDB(t)

	id, err := database.InsertTrigger(TriggerRecord{
		TriggerType:   "commit",
		Timestamp:     time.Now(),
		Source:        "git",
		RepoPath:      "/repo/devtrack",
		CommitHash:    "def5678",
		CommitMessage: "chore: update readme",
		Author:        "dev",
		TicketID:      "",
	})
	if err != nil {
		t.Fatalf("InsertTrigger: %v", err)
	}

	got, err := database.GetTriggerByID(id)
	if err != nil {
		t.Fatalf("GetTriggerByID: %v", err)
	}
	if got.TicketID != "" {
		t.Errorf("TicketID = %q, want empty string for unlinked commit", got.TicketID)
	}
}

// TestGetRecentTriggers_IncludesTicketID confirms the list query also surfaces
// the ticket_id column (not just the single-row lookup).
func TestGetRecentTriggers_IncludesTicketID(t *testing.T) {
	database := newTestDB(t)

	if _, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit",
		Timestamp:   time.Now(),
		Source:      "git",
		RepoPath:    "/repo/devtrack",
		CommitHash:  "111aaaa",
		TicketID:    "AB-7",
	}); err != nil {
		t.Fatalf("InsertTrigger: %v", err)
	}

	triggers, err := database.GetRecentTriggers(10)
	if err != nil {
		t.Fatalf("GetRecentTriggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].TicketID != "AB-7" {
		t.Errorf("TicketID = %q, want %q", triggers[0].TicketID, "AB-7")
	}
}

// TestGetLastTicketID confirms the active-ticket fallback query (used by
// TASK-069) returns the most recently matched ticket for a repo, ignoring
// unlinked (empty) commits, and "" when no prior matched commit exists.
func TestGetLastTicketID(t *testing.T) {
	database := newTestDB(t)

	repoPath := "/repo/devtrack"

	// No commits yet — should return "" with no error.
	last, err := database.GetLastTicketID(repoPath)
	if err != nil {
		t.Fatalf("GetLastTicketID (empty): %v", err)
	}
	if last != "" {
		t.Errorf("GetLastTicketID with no prior commits = %q, want empty", last)
	}

	base := time.Now().Add(-1 * time.Hour)
	inserts := []TriggerRecord{
		{TriggerType: "commit", Timestamp: base, Source: "git", RepoPath: repoPath, CommitHash: "h1", TicketID: "PROJ-1"},
		{TriggerType: "commit", Timestamp: base.Add(10 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h2", TicketID: ""},
		{TriggerType: "commit", Timestamp: base.Add(20 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h3", TicketID: "PROJ-2"},
	}
	for _, r := range inserts {
		if _, err := database.InsertTrigger(r); err != nil {
			t.Fatalf("InsertTrigger: %v", err)
		}
	}

	last, err = database.GetLastTicketID(repoPath)
	if err != nil {
		t.Fatalf("GetLastTicketID: %v", err)
	}
	if last != "PROJ-2" {
		t.Errorf("GetLastTicketID = %q, want %q (most recent matched commit, skipping unlinked h2)", last, "PROJ-2")
	}

	// A different repo path must not see this repo's tickets.
	other, err := database.GetLastTicketID("/repo/other")
	if err != nil {
		t.Fatalf("GetLastTicketID (other repo): %v", err)
	}
	if other != "" {
		t.Errorf("GetLastTicketID for unrelated repo = %q, want empty", other)
	}
}
