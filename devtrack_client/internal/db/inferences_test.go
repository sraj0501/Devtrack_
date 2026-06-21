package db

import (
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newInferencesTestDB creates a temporary SQLite database with the inferences,
// corrections, and confidence_thresholds tables ready for unit testing.
// No environment variables are needed — the database file lives in t.TempDir().
func newInferencesTestDB(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "inferences_test.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}

	testDB := &Database{db: sqlDB, path: dbPath}

	// Create the inferences table (migration 008).
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS inferences (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			context_type TEXT    NOT NULL,
			subject      TEXT    NOT NULL,
			inference    TEXT    NOT NULL,
			evidence     TEXT    NOT NULL,
			confidence   REAL    NOT NULL DEFAULT 0.5,
			source       TEXT    NOT NULL DEFAULT 'hermes3',
			created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create inferences table: %v", err)
	}

	// Create the FTS5 virtual table.
	_, err = sqlDB.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS inferences_fts USING fts5(
			context_type,
			subject,
			inference,
			content='inferences',
			content_rowid='id'
		);
	`)
	if err != nil {
		t.Fatalf("create inferences_fts: %v", err)
	}

	// Create the sync triggers.
	for _, ddl := range []string{
		`CREATE TRIGGER IF NOT EXISTS inferences_ai AFTER INSERT ON inferences BEGIN
			INSERT INTO inferences_fts(rowid, context_type, subject, inference)
			VALUES (new.id, new.context_type, new.subject, new.inference);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS inferences_au AFTER UPDATE ON inferences BEGIN
			INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
			VALUES('delete', old.id, old.context_type, old.subject, old.inference);
			INSERT INTO inferences_fts(rowid, context_type, subject, inference)
			VALUES (new.id, new.context_type, new.subject, new.inference);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS inferences_ad AFTER DELETE ON inferences BEGIN
			INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
			VALUES('delete', old.id, old.context_type, old.subject, old.inference);
		END;`,
	} {
		if _, err := sqlDB.Exec(ddl); err != nil {
			t.Fatalf("create inferences trigger: %v", err)
		}
	}

	// Create the corrections table (migration 009).
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS corrections (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			inference_id  INTEGER NOT NULL REFERENCES inferences(id),
			correction    TEXT    NOT NULL,
			flagged_from  TEXT    NOT NULL DEFAULT 'tui',
			weight        REAL    NOT NULL DEFAULT 2.0,
			created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_corrections_inference ON corrections(inference_id);
	`)
	if err != nil {
		t.Fatalf("create corrections table: %v", err)
	}

	// Create the confidence_thresholds table (migration 010).
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS confidence_thresholds (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			action_type   TEXT    NOT NULL,
			workspace     TEXT    NOT NULL DEFAULT '',
			threshold     REAL    NOT NULL DEFAULT 0.70,
			approvals     INTEGER NOT NULL DEFAULT 0,
			rejections    INTEGER NOT NULL DEFAULT 0,
			last_updated  DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_thresholds_type_ws
			ON confidence_thresholds(action_type, workspace);
	`)
	if err != nil {
		t.Fatalf("create confidence_thresholds table: %v", err)
	}

	t.Cleanup(func() {
		_ = testDB.Close()
		_ = os.Remove(dbPath)
	})
	return testDB
}

// ---------------------------------------------------------------------------
// Test: InsertInference + GetInference round-trip
// ---------------------------------------------------------------------------

func TestInferenceInsertGet(t *testing.T) {
	db := newInferencesTestDB(t)

	inf := Inference{
		ContextType: "commit",
		Subject:     "commit tone",
		Inference:   "developer prefers imperative mood in commit messages",
		Evidence:    `[1, 2, 3]`,
		Confidence:  0.75,
		Source:      "hermes3",
	}

	id, err := db.InsertInference(inf)
	if err != nil {
		t.Fatalf("InsertInference: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, err := db.GetInference(id)
	if err != nil {
		t.Fatalf("GetInference: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.ContextType != inf.ContextType {
		t.Errorf("ContextType: got %q, want %q", got.ContextType, inf.ContextType)
	}
	if got.Subject != inf.Subject {
		t.Errorf("Subject: got %q, want %q", got.Subject, inf.Subject)
	}
	if got.Inference != inf.Inference {
		t.Errorf("Inference: got %q, want %q", got.Inference, inf.Inference)
	}
	if got.Evidence != inf.Evidence {
		t.Errorf("Evidence: got %q, want %q", got.Evidence, inf.Evidence)
	}
	if math.Abs(got.Confidence-inf.Confidence) > 1e-9 {
		t.Errorf("Confidence: got %v, want %v", got.Confidence, inf.Confidence)
	}
	if got.Source != inf.Source {
		t.Errorf("Source: got %q, want %q", got.Source, inf.Source)
	}
}

// ---------------------------------------------------------------------------
// Test: SearchInferences (FTS5) returns the inserted row
// ---------------------------------------------------------------------------

func TestSearchInferences(t *testing.T) {
	db := newInferencesTestDB(t)

	inf := Inference{
		ContextType: "commit",
		Subject:     "commit style",
		Inference:   "developer prefers imperative mood in commit messages",
		Evidence:    `[10]`,
		Confidence:  0.8,
		Source:      "hermes3",
	}
	_, err := db.InsertInference(inf)
	if err != nil {
		t.Fatalf("InsertInference: %v", err)
	}

	results, err := db.SearchInferences("imperative mood", 5)
	if err != nil {
		t.Fatalf("SearchInferences: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchInferences: expected at least 1 result, got 0")
	}
	found := false
	for _, r := range results {
		if r.Inference == inf.Inference {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SearchInferences: expected to find inference %q in results", inf.Inference)
	}
}

// ---------------------------------------------------------------------------
// Test: RecordApproval + RecordRejection threshold formula
// 8 approvals + 2 rejections → threshold = 0.70 + 0.20 * (8/10) = 0.86
// ---------------------------------------------------------------------------

func TestThresholdFormula(t *testing.T) {
	db := newInferencesTestDB(t)

	actionType := "post_comment"
	workspace := ""

	// Record 8 approvals and 2 rejections.
	for i := range 8 {
		if err := db.RecordApproval(actionType, workspace); err != nil {
			t.Fatalf("RecordApproval #%d: %v", i+1, err)
		}
	}
	for i := range 2 {
		if err := db.RecordRejection(actionType, workspace); err != nil {
			t.Fatalf("RecordRejection #%d: %v", i+1, err)
		}
	}

	ct, err := db.GetOrCreateThreshold(actionType, workspace)
	if err != nil {
		t.Fatalf("GetOrCreateThreshold: %v", err)
	}

	want := 0.86 // 0.70 + 0.20 * (8/10)
	if math.Abs(ct.Threshold-want) > 1e-9 {
		t.Errorf("Threshold: got %.10f, want %.10f", ct.Threshold, want)
	}
	if ct.Approvals != 8 {
		t.Errorf("Approvals: got %d, want 8", ct.Approvals)
	}
	if ct.Rejections != 2 {
		t.Errorf("Rejections: got %d, want 2", ct.Rejections)
	}
}

// ---------------------------------------------------------------------------
// Test: GetOrCreateThreshold is idempotent (same ID on second call)
// ---------------------------------------------------------------------------

func TestGetOrCreateThresholdIdempotent(t *testing.T) {
	db := newInferencesTestDB(t)

	actionType := "state_transition"
	workspace := "my-workspace"

	first, err := db.GetOrCreateThreshold(actionType, workspace)
	if err != nil {
		t.Fatalf("GetOrCreateThreshold (first): %v", err)
	}
	if first.ID <= 0 {
		t.Fatalf("expected positive ID on first call, got %d", first.ID)
	}

	second, err := db.GetOrCreateThreshold(actionType, workspace)
	if err != nil {
		t.Fatalf("GetOrCreateThreshold (second): %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("GetOrCreateThreshold: not idempotent — first ID %d, second ID %d", first.ID, second.ID)
	}
}

// ---------------------------------------------------------------------------
// Test: InsertCorrection + ListCorrectionsForInference round-trip
// ---------------------------------------------------------------------------

func TestCorrectionRoundTrip(t *testing.T) {
	db := newInferencesTestDB(t)

	// Insert an inference first (corrections FK references inferences.id).
	infID, err := db.InsertInference(Inference{
		ContextType: "comment",
		Subject:     "comment length",
		Inference:   "developer prefers short, concise comments",
		Evidence:    `[]`,
		Confidence:  0.6,
		Source:      "hermes3",
	})
	if err != nil {
		t.Fatalf("InsertInference: %v", err)
	}

	corr := Correction{
		InferenceID: infID,
		Correction:  "Comments can be longer when context is complex",
		FlaggedFrom: "cli",
		Weight:      2.5,
	}
	corrID, err := db.InsertCorrection(corr)
	if err != nil {
		t.Fatalf("InsertCorrection: %v", err)
	}
	if corrID <= 0 {
		t.Fatalf("expected positive correction ID, got %d", corrID)
	}

	list, err := db.ListCorrectionsForInference(infID)
	if err != nil {
		t.Fatalf("ListCorrectionsForInference: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListCorrectionsForInference: expected 1, got %d", len(list))
	}
	got := list[0]
	if got.ID != corrID {
		t.Errorf("Correction.ID: got %d, want %d", got.ID, corrID)
	}
	if got.InferenceID != infID {
		t.Errorf("InferenceID: got %d, want %d", got.InferenceID, infID)
	}
	if got.Correction != corr.Correction {
		t.Errorf("Correction text: got %q, want %q", got.Correction, corr.Correction)
	}
	if got.FlaggedFrom != corr.FlaggedFrom {
		t.Errorf("FlaggedFrom: got %q, want %q", got.FlaggedFrom, corr.FlaggedFrom)
	}
	if math.Abs(got.Weight-corr.Weight) > 1e-9 {
		t.Errorf("Weight: got %v, want %v", got.Weight, corr.Weight)
	}
}

// ---------------------------------------------------------------------------
// Test: InsertCorrectionRoundTrip — tui path (flagged_from="tui", weight=2.0)
// Verifies the exact fields the TUI submitFlag() writes.
// ---------------------------------------------------------------------------

func TestInsertCorrectionRoundTrip(t *testing.T) {
	db := newInferencesTestDB(t)

	infID, err := db.InsertInference(Inference{
		ContextType: "commit",
		Subject:     "commit tone",
		Inference:   "developer uses imperative mood",
		Evidence:    `[5]`,
		Confidence:  0.9,
		Source:      "hermes3",
	})
	if err != nil {
		t.Fatalf("InsertInference: %v", err)
	}

	corrID, err := db.InsertCorrection(Correction{
		InferenceID: infID,
		Correction:  "actually uses past tense occasionally",
		FlaggedFrom: "tui",
		Weight:      2.0,
	})
	if err != nil {
		t.Fatalf("InsertCorrection: %v", err)
	}
	if corrID <= 0 {
		t.Fatalf("expected positive correction ID, got %d", corrID)
	}

	list, err := db.ListCorrectionsForInference(infID)
	if err != nil {
		t.Fatalf("ListCorrectionsForInference: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 correction, got %d", len(list))
	}
	c := list[0]
	if c.FlaggedFrom != "tui" {
		t.Errorf("FlaggedFrom: got %q, want \"tui\"", c.FlaggedFrom)
	}
	if math.Abs(c.Weight-2.0) > 1e-9 {
		t.Errorf("Weight: got %v, want 2.0", c.Weight)
	}
	if c.Correction != "actually uses past tense occasionally" {
		t.Errorf("Correction text: got %q", c.Correction)
	}
}

// ---------------------------------------------------------------------------
// Test: UpdateInferenceConfidence — halved confidence persists in DB
// ---------------------------------------------------------------------------

func TestUpdateInferenceConfidence(t *testing.T) {
	db := newInferencesTestDB(t)

	infID, err := db.InsertInference(Inference{
		ContextType: "commit",
		Subject:     "commit length",
		Inference:   "developer writes short commit messages",
		Evidence:    `[7]`,
		Confidence:  0.8,
		Source:      "hermes3",
	})
	if err != nil {
		t.Fatalf("InsertInference: %v", err)
	}

	newConf := 0.4 // 0.8 * 0.5
	if err := db.UpdateInferenceConfidence(infID, newConf); err != nil {
		t.Fatalf("UpdateInferenceConfidence: %v", err)
	}

	got, err := db.GetInference(infID)
	if err != nil {
		t.Fatalf("GetInference after update: %v", err)
	}
	if math.Abs(got.Confidence-newConf) > 1e-9 {
		t.Errorf("Confidence after halving: got %v, want %v", got.Confidence, newConf)
	}
}
