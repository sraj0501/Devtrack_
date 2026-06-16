package db

import (
	"testing"
	"time"
)

// TestTicketStats_CountsLinkedAndUnlinked confirms TASK-070's acceptance
// criterion: Database.TicketStats returns correct totals from the triggers
// table, distinguishing linked (non-empty, non-"unlinked" ticket_id) from
// unlinked commits.
func TestTicketStats_CountsLinkedAndUnlinked(t *testing.T) {
	database := newTestDB(t)
	repoPath := "/repo/devtrack"

	base := time.Now().Add(-1 * time.Hour)
	inserts := []TriggerRecord{
		{TriggerType: "commit", Timestamp: base, Source: "git", RepoPath: repoPath, CommitHash: "h1", TicketID: "PROJ-1"},
		{TriggerType: "commit", Timestamp: base.Add(10 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h2", TicketID: ""},
		{TriggerType: "commit", Timestamp: base.Add(20 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h3", TicketID: "PROJ-2"},
		{TriggerType: "commit", Timestamp: base.Add(30 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h4", TicketID: ""},
	}
	for _, r := range inserts {
		if _, err := database.InsertTrigger(r); err != nil {
			t.Fatalf("InsertTrigger: %v", err)
		}
	}

	total, linked, unlinked, err := database.TicketStats(repoPath, 50)
	if err != nil {
		t.Fatalf("TicketStats: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if linked != 2 {
		t.Errorf("linked = %d, want 2", linked)
	}
	if unlinked != 2 {
		t.Errorf("unlinked = %d, want 2", unlinked)
	}
}

// TestTicketStats_RespectsLastN confirms only the most recent N commit
// triggers are considered, not the entire history.
func TestTicketStats_RespectsLastN(t *testing.T) {
	database := newTestDB(t)
	repoPath := "/repo/devtrack"

	base := time.Now().Add(-2 * time.Hour)
	// Oldest 3 are unlinked, newest 2 are linked.
	inserts := []TriggerRecord{
		{TriggerType: "commit", Timestamp: base, Source: "git", RepoPath: repoPath, CommitHash: "h1", TicketID: ""},
		{TriggerType: "commit", Timestamp: base.Add(10 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h2", TicketID: ""},
		{TriggerType: "commit", Timestamp: base.Add(20 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h3", TicketID: ""},
		{TriggerType: "commit", Timestamp: base.Add(30 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h4", TicketID: "PROJ-1"},
		{TriggerType: "commit", Timestamp: base.Add(40 * time.Minute), Source: "git", RepoPath: repoPath, CommitHash: "h5", TicketID: "PROJ-2"},
	}
	for _, r := range inserts {
		if _, err := database.InsertTrigger(r); err != nil {
			t.Fatalf("InsertTrigger: %v", err)
		}
	}

	// Limit to the 2 most recent triggers — both linked.
	total, linked, unlinked, err := database.TicketStats(repoPath, 2)
	if err != nil {
		t.Fatalf("TicketStats: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if linked != 2 {
		t.Errorf("linked = %d, want 2", linked)
	}
	if unlinked != 0 {
		t.Errorf("unlinked = %d, want 0", unlinked)
	}
}

// TestTicketStats_EmptyRepoPathAggregatesAll confirms repoPath="" aggregates
// ticket stats across all workspaces, not just one repo.
func TestTicketStats_EmptyRepoPathAggregatesAll(t *testing.T) {
	database := newTestDB(t)

	base := time.Now().Add(-1 * time.Hour)
	inserts := []TriggerRecord{
		{TriggerType: "commit", Timestamp: base, Source: "git", RepoPath: "/repo/a", CommitHash: "h1", TicketID: "PROJ-1"},
		{TriggerType: "commit", Timestamp: base.Add(10 * time.Minute), Source: "git", RepoPath: "/repo/b", CommitHash: "h2", TicketID: ""},
	}
	for _, r := range inserts {
		if _, err := database.InsertTrigger(r); err != nil {
			t.Fatalf("InsertTrigger: %v", err)
		}
	}

	total, linked, unlinked, err := database.TicketStats("", 50)
	if err != nil {
		t.Fatalf("TicketStats: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if linked != 1 {
		t.Errorf("linked = %d, want 1", linked)
	}
	if unlinked != 1 {
		t.Errorf("unlinked = %d, want 1", unlinked)
	}
}

// TestTicketStats_NoTriggersReturnsZero confirms an empty triggers table
// produces total=0 without error (the "not enough data" case is handled by
// the CLI caller, not this function).
func TestTicketStats_NoTriggersReturnsZero(t *testing.T) {
	database := newTestDB(t)

	total, linked, unlinked, err := database.TicketStats("", 50)
	if err != nil {
		t.Fatalf("TicketStats: %v", err)
	}
	if total != 0 || linked != 0 || unlinked != 0 {
		t.Errorf("got total=%d linked=%d unlinked=%d, want all zero", total, linked, unlinked)
	}
}
