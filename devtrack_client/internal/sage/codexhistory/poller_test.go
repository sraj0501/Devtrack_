package codexhistory

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	_ "modernc.org/sqlite"
)

func createHistoryDatabases(t *testing.T, home string) (*sql.DB, *sql.DB) {
	t.Helper()
	index, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.Exec(`CREATE TABLE threads (id TEXT PRIMARY KEY, source TEXT, archived INTEGER); INSERT INTO threads VALUES ('ide-thread','vscode',0),('cli-thread','cli',0),('other','unknown',0)`); err != nil {
		t.Fatal(err)
	}
	history, err := sql.Open("sqlite", filepath.Join(home, "thread_history_1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.Exec(`CREATE TABLE thread_items (thread_id TEXT, item_id TEXT, rollout_ordinal INTEGER, item_type TEXT, created_at_ms INTEGER, item_json TEXT, PRIMARY KEY(thread_id,item_id))`); err != nil {
		t.Fatal(err)
	}
	return index, history
}

func TestPollStartsFromNowAndCapturesIDEAndCLIOnce(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(1_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("initialize: %d, %v", count, err)
	}
	for ordinal, source := range []string{"ide-thread", "cli-thread", "other"} {
		raw := `{"type":"commandExecution","status":"completed","exitCode":0,"cwd":"C:/private/project","commandActions":[{"command":"git status"}],"output":"CANARY"}`
		if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?)`, source, source+"-item", ordinal, "commandExecution", now.Add(time.Millisecond).UnixMilli(), raw); err != nil {
			t.Fatal(err)
		}
	}
	history.Close()
	count, err := poller.Poll(context.Background())
	if err != nil || count != 2 {
		t.Fatalf("capture: %d, %v", count, err)
	}
	var imported []sage.Event
	result, err := sage.ImportSpool(root, 10, func(event sage.Event) (bool, error) { imported = append(imported, event); return true, nil })
	if err != nil || result.Imported != 2 || len(imported) != 2 {
		t.Fatalf("import: %+v, %d, %v", result, len(imported), err)
	}
	for _, event := range imported {
		if event.Command != "git status" || event.Harness != "codex-history" || event.ProjectID == "C:/private/project" {
			t.Fatalf("unsafe event: %+v", event)
		}
	}
	count, err = poller.Poll(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("repeat: %d, %v", count, err)
	}
}

func TestPollTracksInProgressItemUntilTerminal(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(2_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if _, err := poller.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?)`, "cli-thread", "pending", 1, "commandExecution", now.Add(time.Millisecond).UnixMilli(), `{"type":"commandExecution","status":"inProgress","commandActions":[{"command":"go test ./..."}]}`); err != nil {
		t.Fatal(err)
	}
	history.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("pending: %d, %v", count, err)
	}
	writeDB, err := sql.Open("sqlite", filepath.Join(home, "thread_history_1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeDB.Exec(`UPDATE thread_items SET item_json=? WHERE item_id='pending'`, `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"go test ./..."}]}`); err != nil {
		t.Fatal(err)
	}
	writeDB.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 1 {
		t.Fatalf("terminal: %d, %v", count, err)
	}
}
