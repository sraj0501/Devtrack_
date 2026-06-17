package db

// Tests for CountTicketCommits (TASK-073).
//
// CountTicketCommits answers: "how many prior commit trigger rows reference
// this ticket_id in this repo?" Callers invoke it BEFORE InsertTrigger so the
// returned count reflects rows from previous commits only.

import (
	"testing"
	"time"
)

func TestCountTicketCommits(t *testing.T) {
	database := newTestDB(t)

	repo := "/repo/devtrack"
	otherRepo := "/repo/other"
	ticketID := "PROJ-123"

	// --- 0 prior commits → first commit for this ticket ---
	count, err := database.CountTicketCommits(repo, ticketID)
	if err != nil {
		t.Fatalf("CountTicketCommits (empty): %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 prior commits, got %d", count)
	}

	// Insert 2 commits for PROJ-123 in repo
	base := time.Now().Add(-1 * time.Hour)
	for i, hash := range []string{"aaaaa01", "aaaaa02"} {
		if _, err := database.InsertTrigger(TriggerRecord{
			TriggerType: "commit",
			Timestamp:   base.Add(time.Duration(i) * time.Minute),
			Source:      "git",
			RepoPath:    repo,
			CommitHash:  hash,
			TicketID:    ticketID,
		}); err != nil {
			t.Fatalf("InsertTrigger(%d): %v", i, err)
		}
	}

	// --- 2 prior commits → not first commit ---
	count, err = database.CountTicketCommits(repo, ticketID)
	if err != nil {
		t.Fatalf("CountTicketCommits (2 rows): %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 prior commits, got %d", count)
	}

	// --- Different repo_path must be excluded ---
	if _, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit",
		Timestamp:   base.Add(10 * time.Minute),
		Source:      "git",
		RepoPath:    otherRepo,
		CommitHash:  "bbbbb01",
		TicketID:    ticketID,
	}); err != nil {
		t.Fatalf("InsertTrigger (other repo): %v", err)
	}

	countRepo, err := database.CountTicketCommits(repo, ticketID)
	if err != nil {
		t.Fatalf("CountTicketCommits after other-repo insert: %v", err)
	}
	if countRepo != 2 {
		t.Errorf("other-repo commit must not affect count for repo: got %d, want 2", countRepo)
	}

	countOther, err := database.CountTicketCommits(otherRepo, ticketID)
	if err != nil {
		t.Fatalf("CountTicketCommits (otherRepo): %v", err)
	}
	if countOther != 1 {
		t.Errorf("expected 1 commit in otherRepo, got %d", countOther)
	}

	// --- Empty ticketID always returns 0, no error ---
	countEmpty, err := database.CountTicketCommits(repo, "")
	if err != nil {
		t.Fatalf("CountTicketCommits (empty ticketID): %v", err)
	}
	if countEmpty != 0 {
		t.Errorf("empty ticketID must return 0, got %d", countEmpty)
	}

	// --- Different ticket in same repo is independent ---
	countOtherTicket, err := database.CountTicketCommits(repo, "AB-7")
	if err != nil {
		t.Fatalf("CountTicketCommits (AB-7): %v", err)
	}
	if countOtherTicket != 0 {
		t.Errorf("unrelated ticket must return 0, got %d", countOtherTicket)
	}
}
