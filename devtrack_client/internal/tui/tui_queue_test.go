package tui

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// newQueueTestDB creates a fully functional SQLite DB using NewDatabaseAtPath,
// which applies the base schema plus all migration-managed tables (including
// pending_actions).
func newQueueTestDB(t *testing.T) *db.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "queue_test.db")
	database, err := db.NewDatabaseAtPath(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func insertQueueTestAction(t *testing.T, database *db.Database, payload map[string]any, status string) int64 {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	id, err := database.InsertPendingAction(db.PendingAction{
		ActionType: "post_comment",
		Target:     "PROJ-1",
		Platform:   "github",
		Workspace:  "main-ws",
		Payload:    string(raw),
		Confidence: 0.75,
		Status:     status,
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("InsertPendingAction: %v", err)
	}
	return id
}

func TestExtractCommentField(t *testing.T) {
	if comment, ok := extractCommentField(`{"comment":"hello world","ticket_id":"PROJ-1"}`); !ok || comment != "hello world" {
		t.Fatalf("expected (hello world, true), got (%q, %v)", comment, ok)
	}
	if _, ok := extractCommentField(`{"pr_title":"x","pr_id":"1"}`); ok {
		t.Fatal("expected ok=false for payload with no comment field")
	}
	if _, ok := extractCommentField(`not json`); ok {
		t.Fatal("expected ok=false for unparseable payload")
	}
}

func TestQueueEditEnterPrefillsExistingComment(t *testing.T) {
	database := newQueueTestDB(t)
	id := insertQueueTestAction(t, database, map[string]any{"comment": "original text", "ticket_id": "PROJ-1"}, "pending")

	m := newQueueModel(database, nil)
	m.items = []db.PendingAction{{ID: id, Status: "pending", Payload: `{"comment":"original text","ticket_id":"PROJ-1"}`}}
	m.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	if updated.editingActionID != id {
		t.Fatalf("expected editingActionID=%d, got %d", id, updated.editingActionID)
	}
	if got := updated.editInput.Value(); got != "original text" {
		t.Fatalf("expected prefilled input %q, got %q", "original text", got)
	}
}

func TestQueueEditSubmitUpdatesPayload(t *testing.T) {
	database := newQueueTestDB(t)
	id := insertQueueTestAction(t, database, map[string]any{"comment": "original text", "ticket_id": "PROJ-1"}, "pending")

	m := newQueueModel(database, nil)
	m.items = []db.PendingAction{{ID: id, Status: "pending", Payload: `{"comment":"original text","ticket_id":"PROJ-1"}`}}
	m.cursor = 0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	// Simulate the user clearing the prefilled text and typing a correction.
	m.editInput.SetValue("")
	for _, r := range "corrected text" {
		m.editInput, _ = m.editInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.editingActionID != 0 {
		t.Fatalf("expected editing mode to close, editingActionID=%d", m.editingActionID)
	}
	if m.editErrMsg != "" {
		t.Fatalf("expected no error, got %q", m.editErrMsg)
	}

	got, err := database.GetPendingAction(id)
	if err != nil {
		t.Fatalf("GetPendingAction: %v", err)
	}
	comment, ok := extractCommentField(got.Payload)
	if !ok || comment != "corrected text" {
		t.Fatalf("expected persisted comment %q, got (%q, %v)", "corrected text", comment, ok)
	}
}

func TestQueueEditEscCancelsWithoutSaving(t *testing.T) {
	database := newQueueTestDB(t)
	id := insertQueueTestAction(t, database, map[string]any{"comment": "original text", "ticket_id": "PROJ-1"}, "pending")

	m := newQueueModel(database, nil)
	m.items = []db.PendingAction{{ID: id, Status: "pending", Payload: `{"comment":"original text","ticket_id":"PROJ-1"}`}}
	m.cursor = 0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.editInput.SetValue("this should be discarded")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.editingActionID != 0 {
		t.Fatalf("expected editing mode to close on Esc, editingActionID=%d", m.editingActionID)
	}

	got, err := database.GetPendingAction(id)
	if err != nil {
		t.Fatalf("GetPendingAction: %v", err)
	}
	comment, _ := extractCommentField(got.Payload)
	if comment != "original text" {
		t.Fatalf("expected payload unchanged after Esc, got comment=%q", comment)
	}
}

func TestQueueEditRejectsNonPendingAction(t *testing.T) {
	database := newQueueTestDB(t)
	id := insertQueueTestAction(t, database, map[string]any{"comment": "already posted"}, "posted")

	m := newQueueModel(database, nil)
	m.editingActionID = id
	m.editInput.SetValue("attempted edit")

	m, _ = m.submitEdit()

	if m.editErrMsg == "" {
		t.Fatal("expected an error message for editing a non-pending action")
	}
	if m.editingActionID != id {
		t.Fatal("expected editing mode to remain open so the error is visible")
	}
}

func TestQueueEditRejectsActionWithNoCommentField(t *testing.T) {
	database := newQueueTestDB(t)
	id := insertQueueTestAction(t, database, map[string]any{"pr_title": "x", "pr_id": "1"}, "pending")

	m := newQueueModel(database, nil)
	m.editingActionID = id
	m.editInput.SetValue("attempted edit")

	m, _ = m.submitEdit()

	if m.editErrMsg == "" {
		t.Fatal("expected an error message for an action type with no editable comment field")
	}
}
