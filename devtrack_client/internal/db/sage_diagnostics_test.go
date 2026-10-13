package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSageDiagnosticsDistinguishesDurableStates(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Unix(100, 0)
	jobs := []struct {
		sig, state string
		attempts   int
		lease      int64
	}{
		{"waiting command", "waiting", 0, 0},
		{"retry command CANARY", "retrying", 2, 0},
		{"processing command", "processing", 1, now.Add(-time.Second).UnixMilli()},
		{"distilled command", "distilled", 1, 0},
		{"skipped command", "skipped", 1, 0},
		{"failed command CANARY", "retrying", SageMaxAttempts, 0},
		{"publication failed CANARY", "distilled", 1, 0},
	}
	for _, job := range jobs {
		if _, err := database.db.Exec(`INSERT INTO sage_jobs
			(signature,command,failed,state,attempts,lease_until_ms,last_error,skip_reason,draft_json)
			VALUES(?,?,0,?,?,?,'model or persistence failure','model judged command not useful','{}')`,
			job.sig, job.sig, job.state, job.attempts, job.lease); err != nil {
			t.Fatal(err)
		}
	}
	for _, publication := range []struct {
		sig, state string
		attempts   int
	}{
		{"distilled command", "documented", 1},
		{"skipped command", "waiting", 0},
		{"publication failed CANARY", "retrying", SageMaxAttempts},
	} {
		if _, err := database.db.Exec(`INSERT INTO sage_publications(signature,state,attempts,last_error)
			VALUES(?,?,?,'knowledge publication failure')`, publication.sig, publication.state, publication.attempts); err != nil {
			t.Fatal(err)
		}
	}

	status, err := database.SageDiagnostics(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if status.Waiting != 1 || status.Retrying != 1 || status.Processing != 1 || status.ExpiredLeases != 1 ||
		status.Distilled != 2 || status.Skipped != 1 || status.PublicationQueued != 1 ||
		status.Documented != 1 || status.TerminalFailed != 2 {
		t.Fatalf("status=%+v", status)
	}
	entries, err := database.SageLog(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "CANARY") || strings.Contains(string(raw), "retry command") {
		t.Fatalf("private command leaked: %s", raw)
	}
	terminal := 0
	for _, entry := range entries {
		if len(entry.ID) != 12 {
			t.Fatalf("unsafe diagnostic id: %+v", entry)
		}
		if entry.State == "terminally_failed" {
			terminal++
		}
	}
	if terminal != 2 {
		t.Fatalf("terminal entries=%d: %+v", terminal, entries)
	}
}

func TestSageRetriesStopAfterBoundedAttempts(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.db.Exec(`INSERT INTO sage_jobs(signature,command,failed,state,attempts)
		VALUES('job','private command',0,'retrying',?)`, SageMaxAttempts); err != nil {
		t.Fatal(err)
	}
	queue := SageQueue{Database: database, Now: func() time.Time { return time.Unix(100, 0) }}
	if _, found, err := queue.Claim(context.Background()); err != nil || found {
		t.Fatalf("terminal job reclaimed: found=%v err=%v", found, err)
	}
	if _, err := database.db.Exec(`UPDATE sage_jobs SET state='distilled',draft_json='{}' WHERE signature='job'`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`UPDATE sage_publications SET state='retrying',attempts=? WHERE signature='job'`, SageMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if found, err := queue.PublishNext(context.Background(), nil); err != nil || found {
		t.Fatalf("terminal publication reclaimed: found=%v err=%v", found, err)
	}
}
