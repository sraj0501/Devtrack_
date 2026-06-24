package mcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// openTestDB creates a fresh SQLite DB in a temp dir without requiring any
// environment variables. Uses db.NewDatabaseAtPath which bypasses config.
// Returns the DB and a cleanup function.
func openTestDB(t *testing.T) (*db.Database, func()) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "devtrack_test.db")
	database, err := db.NewDatabaseAtPath(dbPath)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	return database, func() {
		database.Close()
	}
}

func TestGetActiveContext_NoCommits(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	handler := makeGetActiveContext(database)
	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["confidence"] != "none" {
		t.Errorf("expected confidence=none on empty DB, got %v", m["confidence"])
	}
	if m["active_ticket"] != "" {
		t.Errorf("expected active_ticket empty on empty DB, got %v", m["active_ticket"])
	}
}

func TestGetActiveContext_WithTicket(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	// Insert a commit trigger directly via SQL
	_, err := database.ExecRaw(`
		INSERT INTO triggers (trigger_type, commit_hash, commit_message, ticket_id, repo_path, timestamp, source)
		VALUES ('commit', 'abc12345', 'fix auth flow', 'PROJ-123', '/repos/devtrack', datetime('now'), 'git')
	`)
	if err != nil {
		t.Fatalf("insert test commit: %v", err)
	}

	handler := makeGetActiveContext(database)
	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["active_ticket"] != "PROJ-123" {
		t.Errorf("expected active_ticket=PROJ-123, got %v", m["active_ticket"])
	}
	if m["confidence"] != "high" {
		t.Errorf("expected confidence=high, got %v", m["confidence"])
	}
}

func TestGetTodayCommits_Groups(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	// Insert two commits for the same ticket
	for i, msg := range []string{"commit one", "commit two"} {
		hash := "aaaaaaa" + string(rune('0'+i))
		_, err := database.ExecRaw(`
			INSERT INTO triggers (trigger_type, commit_hash, commit_message, ticket_id, repo_path, timestamp, source)
			VALUES ('commit', ?, ?, 'PROJ-99', '/repos/ws', datetime('now'), 'git')
		`, hash, msg)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	handler := makeGetTodayCommits(database)
	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	grouped, _ := m["commits_by_ticket"].(map[string][]map[string]interface{})
	if len(grouped["PROJ-99"]) != 2 {
		t.Errorf("expected 2 commits under PROJ-99, got %v", grouped["PROJ-99"])
	}
	if m["total_today"].(int) != 2 {
		t.Errorf("expected total_today=2, got %v", m["total_today"])
	}
}

func TestGetVoiceProfile_NoData(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	handler := makeGetVoiceProfile(database)
	result, err := handler(context.Background(), map[string]interface{}{"context_type": "commit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	infs, _ := m["inferences"].([]map[string]interface{})
	if len(infs) != 0 {
		t.Errorf("expected empty inferences on empty DB, got %v", infs)
	}
	if m["note"] == nil || m["note"] == "" {
		t.Errorf("expected note field when empty, got nil/empty")
	}
}

func TestGetTicketContext_Filters(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	// Insert commits for two different tickets
	for _, row := range []struct{ hash, ticket string }{
		{"aaa00001", "PROJ-1"},
		{"bbb00002", "PROJ-2"},
		{"ccc00003", "PROJ-1"},
	} {
		_, err := database.ExecRaw(`
			INSERT INTO triggers (trigger_type, commit_hash, commit_message, ticket_id, repo_path, timestamp, source)
			VALUES ('commit', ?, 'msg', ?, '/repos/ws', datetime('now'), 'git')
		`, row.hash, row.ticket)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	handler := makeGetTicketContext(database)
	result, err := handler(context.Background(), map[string]interface{}{"ticket_id": "PROJ-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	commits, _ := m["recent_commits"].([]map[string]interface{})
	if len(commits) != 2 {
		t.Errorf("expected 2 commits for PROJ-1, got %d", len(commits))
	}
}

func TestGetPendingActions_Empty(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	handler := makeGetPendingActions(database)
	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["count"].(int) != 0 {
		t.Errorf("expected count=0 on empty DB, got %v", m["count"])
	}
}

func TestGetEODSummary_NoCommits(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	handler := makeGetEODSummary(database)
	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["summary"] != "No commits today." {
		t.Errorf("expected 'No commits today.' summary, got %v", m["summary"])
	}
	if m["total_commits"].(int) != 0 {
		t.Errorf("expected total_commits=0, got %v", m["total_commits"])
	}
}

func TestRegisterDevTrackTools(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	srv := New("test-v0")
	RegisterDevTrackTools(srv, database)

	// Verify all 6 tools are registered by checking the tool list
	tools := srv.toolList()
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool["name"].(string)] = true
	}

	expected := []string{
		"get_active_context",
		"get_today_commits",
		"get_pending_actions",
		"get_voice_profile",
		"get_ticket_context",
		"get_eod_summary",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q to be registered, but it was not", name)
		}
	}
	if len(tools) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(tools))
	}
}
