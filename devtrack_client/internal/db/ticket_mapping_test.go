package db

import (
	"sync"
	"testing"
	"time"
)

func TestTriggerMappingProvenanceRoundTrip(t *testing.T) {
	database := newTestDB(t)
	id, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git",
		RepoPath: "/repo", CommitHash: "abc", CommitMessage: "work", TicketID: "GH-42",
		TicketCanonicalRef: "GH-42", TicketExternalID: "42", TicketSource: "branch",
		TicketConfidence: 1, TicketState: "linked", TicketBranch: "feature/GH-42-work",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := database.GetTriggerByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.TicketCanonicalRef != "GH-42" || got.TicketExternalID != "42" ||
		got.TicketSource != "branch" || got.TicketState != "linked" || got.TicketConfidence != 1 {
		t.Fatalf("mapping provenance = %#v", got)
	}
}

func TestTicketMappingCorrectionConcurrentIdempotency(t *testing.T) {
	database := newTestDB(t)
	database.db.SetMaxOpenConns(1)
	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git", TicketState: "unlinked",
	})
	if err != nil {
		t.Fatal(err)
	}
	correction := TicketMappingCorrection{
		TriggerID: triggerID, ReplacementRef: "GH-42", Channel: "cli", Actor: "user", Reason: "explicit link",
	}
	const workers = 8
	ids := make(chan int64, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, insertErr := database.InsertTicketMappingCorrection(correction)
			ids <- id
			errs <- insertErr
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for insertErr := range errs {
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	var first int64
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id != first {
			t.Fatalf("concurrent correction IDs differ: got %d and %d", first, id)
		}
	}
	var count int
	if err := database.db.QueryRow(`SELECT COUNT(*) FROM ticket_mapping_corrections WHERE trigger_id=?`, triggerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("correction rows = %d, want 1", count)
	}
}

func TestLegacyMappingBackfillPreservesHistoricalTicket(t *testing.T) {
	database := newTestDB(t)
	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git", TicketID: "OLD-7",
		CommitMessage: "NEW-99: historical prose", TicketBranch: "feature/NEW-99-new-rule",
		TicketSource: "legacy", TicketState: "legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.backfillLegacyTicketMappings(); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetTriggerByID(triggerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TicketID != "OLD-7" || stored.TicketCanonicalRef != "OLD-7" ||
		stored.TicketSource != "legacy" || stored.TicketState != "legacy" || stored.TicketConfidence != 0 {
		t.Fatalf("legacy mapping was reinterpreted: %#v", stored)
	}
}

func TestCorrectedMappingIsEffectiveInReadModelsAndHealth(t *testing.T) {
	database := newTestDB(t)
	database.db.SetMaxOpenConns(1)
	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git", CommitHash: "corrected-hash",
		TicketState: "conflict", TicketConflict: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.InsertTicketMappingCorrection(TicketMappingCorrection{
		TriggerID: triggerID, ReplacementRef: "GH-42", Channel: "cli", Actor: "user",
	}); err != nil {
		t.Fatal(err)
	}
	commits, err := database.ListTicketCommits("GH-42", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].TicketID != "GH-42" || commits[0].TicketExternalID != "42" ||
		commits[0].TicketSource != "correction" || commits[0].TicketState != "corrected" || commits[0].TicketConflict {
		t.Fatalf("corrected read model = %#v", commits)
	}
	unlinked, conflicts, err := database.TicketMappingHealth("", 50)
	if err != nil {
		t.Fatal(err)
	}
	if unlinked != 0 || conflicts != 0 {
		t.Fatalf("corrected health = unlinked %d conflicts %d", unlinked, conflicts)
	}
}

func TestTicketMappingCorrectionsAreAppendOnlyAndIdempotent(t *testing.T) {
	database := newTestDB(t)
	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git", TicketID: "PROJ-1",
		TicketCanonicalRef: "PROJ-1", TicketSource: "branch", TicketState: "linked",
	})
	if err != nil {
		t.Fatal(err)
	}
	correction := TicketMappingCorrection{
		TriggerID: triggerID, PreviousRef: "PROJ-1", ReplacementRef: "PROJ-2",
		Channel: "cli", Actor: "user", Reason: "wrong ticket",
	}
	first, err := database.InsertTicketMappingCorrection(correction)
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.InsertTicketMappingCorrection(correction)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("duplicate correction IDs = %d and %d", first, second)
	}
	original, effective, err := database.GetEffectiveTicketMapping(triggerID)
	if err != nil {
		t.Fatal(err)
	}
	if original != "PROJ-1" || effective != "PROJ-2" {
		t.Fatalf("mapping = original %q effective %q", original, effective)
	}
	stored, err := database.GetTriggerByID(triggerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TicketCanonicalRef != "PROJ-1" {
		t.Fatalf("original mapping mutated: %#v", stored)
	}
}

func TestTicketMappingCandidateDoesNotBecomeEffective(t *testing.T) {
	database := newTestDB(t)
	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now(), Source: "git", TicketState: "unlinked",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.InsertTicketMappingCandidate(TicketMappingCandidate{
		TriggerID: triggerID, CandidateRef: "PROJ-9", Confidence: 0.82,
		Source: "llm", Model: "local-test",
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetTriggerByID(triggerID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TicketID != "" || stored.TicketCanonicalRef != "" {
		t.Fatalf("candidate changed effective mapping: %#v", stored)
	}
}
