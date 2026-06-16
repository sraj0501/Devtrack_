package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Unit tests — ConfidenceTimeout (pure function, no DB)
// ---------------------------------------------------------------------------

func TestConfidenceTimeout(t *testing.T) {
	cases := []struct {
		name            string
		confidence      float64
		isNewActionType bool
		want            time.Duration
	}{
		{
			name:            "new action type always 30m",
			confidence:      0.99,
			isNewActionType: true,
			want:            30 * time.Minute,
		},
		{
			name:            "high confidence (>0.90) gets 2m",
			confidence:      0.95,
			isNewActionType: false,
			want:            2 * time.Minute,
		},
		{
			name:            "medium confidence (>=0.70) gets 5m",
			confidence:      0.75,
			isNewActionType: false,
			want:            5 * time.Minute,
		},
		{
			name:            "low confidence (<0.70) gets 15m",
			confidence:      0.50,
			isNewActionType: false,
			want:            15 * time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ConfidenceTimeout(tc.confidence, tc.isNewActionType)
			if got != tc.want {
				t.Errorf("ConfidenceTimeout(%v, %v) = %v; want %v",
					tc.confidence, tc.isNewActionType, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration tests — CRUD against a real (temp) SQLite database
// ---------------------------------------------------------------------------

// newTestDB opens a fresh SQLite database in a temp directory, initialises the
// schema, and registers cleanup to remove the file after the test finishes.
func newTestDB(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_devtrack.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}

	testDB := &Database{db: sqlDB, path: dbPath}
	if err := testDB.initSchema(); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	// Create the pending_actions table directly (migration 006 SQL).
	// In production this is created by RunPendingMigrations; in tests we run it inline.
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS pending_actions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			action_type TEXT    NOT NULL,
			target      TEXT    NOT NULL,
			platform    TEXT    NOT NULL,
			workspace   TEXT    NOT NULL,
			payload     TEXT    NOT NULL,
			confidence  REAL    NOT NULL,
			status      TEXT    NOT NULL DEFAULT 'pending',
			expires_at  DATETIME NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			acted_at    DATETIME,
			acted_by    TEXT,
			error       TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
		CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at);
	`)
	if err != nil {
		t.Fatalf("create pending_actions table: %v", err)
	}

	t.Cleanup(func() {
		_ = testDB.Close()
		_ = os.Remove(dbPath)
	})
	return testDB
}

func TestPendingActionCRUD(t *testing.T) {
	db := newTestDB(t)

	// ---- Insert ----
	expiresAt := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)
	action := PendingAction{
		ActionType: "post_comment",
		Target:     "PROJ-123",
		Platform:   "github",
		Workspace:  "main-ws",
		Payload:    `{"body":"Fixed null check"}`,
		Confidence: 0.82,
		Status:     "pending",
		ExpiresAt:  expiresAt,
	}

	id, err := db.InsertPendingAction(action)
	if err != nil {
		t.Fatalf("InsertPendingAction: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// ---- GetPendingAction ----
	got, err := db.GetPendingAction(id)
	if err != nil {
		t.Fatalf("GetPendingAction: %v", err)
	}
	if got == nil {
		t.Fatalf("GetPendingAction returned nil for id %d", id)
	}
	if got.ActionType != action.ActionType {
		t.Errorf("ActionType: got %q, want %q", got.ActionType, action.ActionType)
	}
	if got.Target != action.Target {
		t.Errorf("Target: got %q, want %q", got.Target, action.Target)
	}
	if got.Status != "pending" {
		t.Errorf("Status: got %q, want %q", got.Status, "pending")
	}
	if got.ActedAt != nil {
		t.Errorf("ActedAt should be nil for a fresh pending action, got %v", got.ActedAt)
	}

	// ---- ListPendingActions (filter by status) ----
	list, err := db.ListPendingActions("pending")
	if err != nil {
		t.Fatalf("ListPendingActions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 pending action, got %d", len(list))
	}
	if list[0].ID != id {
		t.Errorf("listed action ID mismatch: got %d, want %d", list[0].ID, id)
	}

	// ---- UpdatePendingActionStatus ----
	if err := db.UpdatePendingActionStatus(id, "approved", "tui"); err != nil {
		t.Fatalf("UpdatePendingActionStatus: %v", err)
	}

	updated, err := db.GetPendingAction(id)
	if err != nil {
		t.Fatalf("GetPendingAction after update: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("Status after update: got %q, want %q", updated.Status, "approved")
	}
	if updated.ActedBy == nil || *updated.ActedBy != "tui" {
		t.Errorf("ActedBy: got %v, want \"tui\"", updated.ActedBy)
	}
	if updated.ActedAt == nil {
		t.Error("ActedAt should be non-nil after status update")
	}

	// ---- UpdatePendingActionError ----
	id2, err := db.InsertPendingAction(PendingAction{
		ActionType: "state_transition",
		Target:     "ADO-789",
		Platform:   "azure",
		Workspace:  "main-ws",
		Payload:    `{"state":"In Progress"}`,
		Confidence: 0.55,
		Status:     "pending",
		ExpiresAt:  time.Now().Add(15 * time.Minute).UTC(),
	})
	if err != nil {
		t.Fatalf("InsertPendingAction (2): %v", err)
	}

	if err := db.UpdatePendingActionError(id2, "timeout posting to azure"); err != nil {
		t.Fatalf("UpdatePendingActionError: %v", err)
	}

	errored, err := db.GetPendingAction(id2)
	if err != nil {
		t.Fatalf("GetPendingAction after error update: %v", err)
	}
	if errored.Status != "failed" {
		t.Errorf("Status after error: got %q, want %q", errored.Status, "failed")
	}
	if errored.Error == nil || *errored.Error != "timeout posting to azure" {
		t.Errorf("Error field: got %v, want %q", errored.Error, "timeout posting to azure")
	}

	// ---- UpdatePendingActionPayload ----
	newPayload := `{"body":"Updated comment text"}`
	if err := db.UpdatePendingActionPayload(id, newPayload); err != nil {
		t.Fatalf("UpdatePendingActionPayload: %v", err)
	}
	after, err := db.GetPendingAction(id)
	if err != nil {
		t.Fatalf("GetPendingAction after payload update: %v", err)
	}
	if after.Payload != newPayload {
		t.Errorf("Payload after update: got %q, want %q", after.Payload, newPayload)
	}

	// ---- ListPendingActions (no filter — all statuses) ----
	all, err := db.ListPendingActions("")
	if err != nil {
		t.Fatalf("ListPendingActions (all): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 total actions, got %d", len(all))
	}

	// ---- ListPendingActionsRecent ----
	recent, err := db.ListPendingActionsRecent(1)
	if err != nil {
		t.Fatalf("ListPendingActionsRecent: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("expected 2 recent actions (within 1h), got %d", len(recent))
	}

	// ---- Invalid status rejected ----
	if err := db.UpdatePendingActionStatus(id, "unknown_status", "test"); err == nil {
		t.Error("expected error for invalid status, got nil")
	}

	// ---- GetPendingAction for missing ID returns nil, nil ----
	missing, err := db.GetPendingAction(9999)
	if err != nil {
		t.Fatalf("GetPendingAction(missing): unexpected error %v", err)
	}
	if missing != nil {
		t.Errorf("GetPendingAction(missing): expected nil, got %+v", missing)
	}
}
