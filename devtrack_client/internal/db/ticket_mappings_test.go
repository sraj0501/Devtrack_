package db

import (
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

func insertMappedCommit(t *testing.T, d *Database) int64 {
	t.Helper()
	c := ticket.Contract{Key: "PROJ", Provider: "github"}
	r, err := ticket.NewResolver(c)
	if err != nil {
		t.Fatal(err)
	}
	e := ticket.Evidence{Branch: "fix/PROJ-12-login", Message: "PROJ-13: conflicting", ActiveTicket: "PROJ-14"}
	m := r.Resolve(e)
	id, err := d.InsertTrigger(TriggerRecord{TriggerType: "commit", Timestamp: time.Now(), Source: "git", RepoPath: "/repo", CommitHash: "abcdef012345", CommitMessage: e.Message, TicketID: "IGNORED-99", WorkspaceName: "test", TicketMapping: &m, TicketContract: c, TicketEvidence: e})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTicketMappingAtomicPersistenceAndCorrection(t *testing.T) {
	d := newTestDB(t)
	id := insertMappedCommit(t, d)
	m, err := d.GetTicketMapping(id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Effective.TicketID != "PROJ-12" || m.Effective.ExternalID != "12" || m.Effective.State != "conflict" || m.Evidence.ActiveTicket != "PROJ-14" {
		t.Fatalf("mapping=%+v", m)
	}
	req := CorrectTicketRequest{TriggerID: id, Reference: "PROJ-21", Contract: m.Contract, Actor: "local-user", Channel: "cli", RequestID: "request-1", Reason: "wrong branch", ExpectedVersion: 0}
	corrected, err := d.CorrectTicketMapping(req)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Original.TicketID != "PROJ-12" || corrected.Effective.TicketID != "PROJ-21" || corrected.Effective.ExternalID != "21" || corrected.Effective.Conflict || corrected.Effective.State != "corrected" || corrected.Version != 1 {
		t.Fatalf("corrected=%+v", corrected)
	}
	if _, err := d.CorrectTicketMapping(req); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	history, err := d.ListTicketCorrections(id)
	if err != nil || len(history) != 1 || history[0].Previous.TicketID != "PROJ-12" || history[0].Replacement.TicketID != "PROJ-21" {
		t.Fatalf("audit=%+v err=%v", history, err)
	}
	stored, err := d.GetTriggerByID(id)
	if err != nil || stored.TicketID != "PROJ-21" {
		t.Fatalf("effective read model=%+v %v", stored, err)
	}
	commits, err := d.ListTodayCommits("/repo")
	if err != nil || len(commits) != 1 || commits[0].Mapping.Source != "correction" || commits[0].Mapping.ExternalID != "21" || commits[0].TicketID != "PROJ-21" {
		t.Fatalf("commit read model lost effective provenance: %+v %v", commits, err)
	}
	req.Reference = "PROJ-22"
	if _, err := d.CorrectTicketMapping(req); err == nil {
		t.Fatal("request ID reused for different correction")
	}
	for _, query := range []string{`UPDATE ticket_mapping_corrections SET actor='rewritten'`, `DELETE FROM ticket_mapping_corrections`, `UPDATE ticket_mappings SET original_json='{}'`, `UPDATE ticket_mappings SET evidence_json='{}'`} {
		if _, err := d.ExecRaw(query); err == nil {
			t.Fatalf("immutable audit accepted %s", query)
		}
	}
}

func TestTicketMigrationPreservesLegacyAndCorrectionsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-contract row; its incidental message must never be resolved.
	_, err = d.ExecRaw(`INSERT INTO triggers(trigger_type,timestamp,source,repo_path,commit_hash,commit_message,ticket_id) VALUES('commit',?,'git','/repo','legacyhash','PROJ-999: misleading new evidence','old-provider-17')`, time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	id := insertMappedCommit(t, d)
	_, err = d.CorrectTicketMapping(CorrectTicketRequest{TriggerID: id, Reference: "PROJ-77", Contract: ticket.Contract{Key: "PROJ", Provider: "github"}, Actor: "owner", Channel: "telegram", RequestID: "reopen", ExpectedVersion: 0})
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	legacy, err := d.GetTicketMapping(1)
	if err != nil || legacy.Effective.TicketID != "old-provider-17" || legacy.Effective.State != "legacy" || legacy.Effective.Confidence != 0 || legacy.Effective.ExternalID != "" {
		t.Fatalf("legacy reinterpreted: %+v %v", legacy, err)
	}
	corrected, err := d.GetTicketMapping(id)
	if err != nil || corrected.Original.TicketID != "PROJ-12" || corrected.Effective.TicketID != "PROJ-77" {
		t.Fatalf("correction lost: %+v %v", corrected, err)
	}
}

func TestConcurrentTicketCorrectionsRequireFreshVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	first, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	id := insertMappedCommit(t, first)
	second, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for i, d := range []*Database{first, second} {
		wg.Add(1)
		go func(i int, d *Database) {
			defer wg.Done()
			<-start
			ref := []string{"PROJ-20", "PROJ-30"}[i]
			_, err := d.CorrectTicketMapping(CorrectTicketRequest{TriggerID: id, Reference: ref, Contract: ticket.Contract{Key: "PROJ", Provider: "github"}, Actor: "user", Channel: "cli", RequestID: ref, ExpectedVersion: 0})
			results <- err
		}(i, d)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrMappingChanged) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	history, err := first.ListTicketCorrections(id)
	if err != nil || len(history) != 1 {
		t.Fatalf("audit=%+v %v", history, err)
	}
}

func TestMappingFailureRollsBackCommit(t *testing.T) {
	d := newTestDB(t)
	_, err := d.InsertTrigger(TriggerRecord{TriggerType: "commit", Source: "git", Timestamp: time.Now(), CommitHash: "bad", TicketMapping: &ticket.Resolution{Confidence: math.NaN()}})
	if err == nil {
		t.Fatal("invalid mapping accepted")
	}
	var count int
	if err := d.DB().QueryRow(`SELECT COUNT(*) FROM triggers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial commit retained: %d %v", count, err)
	}
}

func TestCommitCorrectionLookupIsRepositoryScopedAndUnambiguous(t *testing.T) {
	d := newTestDB(t)
	id := insertMappedCommit(t, d)
	got, err := d.FindCommitTrigger("/repo", "abcdef0")
	if err != nil || got != id {
		t.Fatalf("lookup=%d %v", got, err)
	}
	if _, err = d.FindCommitTrigger("/other", "abcdef0"); err == nil {
		t.Fatal("cross-repository correction allowed")
	}
	if _, err = d.FindCommitTrigger("/repo", "abcdef%"); err == nil {
		t.Fatal("wildcard hash accepted")
	}
	insertMappedCommit(t, d)
	if _, err = d.FindCommitTrigger("/repo", "abcdef0"); err == nil {
		t.Fatal("ambiguous correction guessed")
	}
}
