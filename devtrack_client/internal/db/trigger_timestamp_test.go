package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTriggerTodayQueriesAcceptGoTimeValues(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "devtrack.db"))
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	defer database.Close()

	now := time.Now()
	if _, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit",
		Timestamp:   now,
		Source:      "git",
		CommitHash:  "abc123",
		TicketID:    "TEST-1",
	}); err != nil {
		t.Fatalf("InsertTrigger: %v", err)
	}

	commits, err := database.ListTodayCommits("")
	if err != nil {
		t.Fatalf("ListTodayCommits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("ListTodayCommits returned %d commits, want 1", len(commits))
	}
	if got, err := database.CountTodayCommits(); err != nil || got != 1 {
		t.Fatalf("CountTodayCommits = %d, %v; want 1, nil", got, err)
	}
	commitCount, timerCount := database.CountTriggersToday()
	if commitCount != 1 || timerCount != 0 {
		t.Fatalf("CountTriggersToday = (%d, %d), want (1, 0)", commitCount, timerCount)
	}
}

func TestTriggerTodayQueriesAcceptLegacyDriverTimestamp(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "devtrack.db"))
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	defer database.Close()

	legacy := time.Now().Format("2006-01-02 15:04:05 -0700 MST")
	if _, err := database.ExecRaw(`
		INSERT INTO triggers (trigger_type, timestamp, source, commit_hash)
		VALUES ('commit', ?, 'git', 'legacy123')`, legacy); err != nil {
		t.Fatalf("insert legacy timestamp: %v", err)
	}

	if got, err := database.CountTodayCommits(); err != nil || got != 1 {
		t.Fatalf("CountTodayCommits = %d, %v; want 1, nil", got, err)
	}
}
