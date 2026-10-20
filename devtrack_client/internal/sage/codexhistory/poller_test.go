package codexhistory

import (
	"context"
	"database/sql"
	"errors"
	"os"
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

func TestReferenceInProgressCommandIsCollectedWhenItFinishes(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(3_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("initialize: %d, %v", count, err)
	}
	slow := `{"type":"commandExecution","status":"inProgress","exitCode":0,"commandActions":[{"command":"go test ./..."}]}`
	fast := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"git status --short"}]}`
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?),(?,?,?,?,?,?)`,
		"cli-thread", "slow", 1, "commandExecution", now.Add(time.Millisecond).UnixMilli(), slow,
		"cli-thread", "fast", 2, "commandExecution", now.Add(2*time.Millisecond).UnixMilli(), fast); err != nil {
		t.Fatal(err)
	}
	history.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 1 {
		t.Fatalf("fast poll: %d, %v", count, err)
	}
	writeDB, err := sql.Open("sqlite", filepath.Join(home, "thread_history_1.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	terminal := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"go test ./..."}]}`
	if _, err := writeDB.Exec(`UPDATE thread_items SET item_json=? WHERE item_id='slow'`, terminal); err != nil {
		t.Fatal(err)
	}
	writeDB.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 1 {
		t.Fatalf("slow terminal poll: %d, %v", count, err)
	}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("repeat poll: %d, %v", count, err)
	}
	var imported []sage.Event
	result, err := sage.ImportSpool(root, 10, func(event sage.Event) (bool, error) {
		imported = append(imported, event)
		return true, nil
	})
	if err != nil || result.Imported != 2 || len(imported) != 2 {
		t.Fatalf("import=%+v events=%d err=%v", result, len(imported), err)
	}
}

func TestReferenceFailedCommandPreservesExitStatus(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(4_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if _, err := poller.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	failed := `{"type":"commandExecution","status":"failed","exitCode":7,"commandActions":[{"command":"git status"}]}`
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?)`, "cli-thread", "failed", 1, "commandExecution", now.Add(time.Millisecond).UnixMilli(), failed); err != nil {
		t.Fatal(err)
	}
	history.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 1 {
		t.Fatalf("poll: %d, %v", count, err)
	}
	var captured sage.Event
	result, err := sage.ImportSpool(root, 10, func(event sage.Event) (bool, error) {
		captured = event
		return true, nil
	})
	if err != nil || result.Imported != 1 || captured.Success == nil || *captured.Success || captured.ExitCode == nil || *captured.ExitCode != 7 {
		t.Fatalf("result=%+v event=%+v err=%v", result, captured, err)
	}
}

func TestReferenceCapturesOriginalCommandOnceAndExcludesCLIAndOldHistory(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(5_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("initialize: %d, %v", count, err)
	}
	old := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"git log --oneline"}]}`
	ide := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"rg -n -i -C 5 bisect knowledge/git.md"}]}`
	cli := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"git status --short"}]}`
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?),(?,?,?,?,?,?),(?,?,?,?,?,?)`,
		"ide-thread", "old", 0, "commandExecution", now.Add(-time.Millisecond).UnixMilli(), old,
		"ide-thread", "new", 1, "commandExecution", now.Add(time.Millisecond).UnixMilli(), ide,
		"cli-thread", "cli", 2, "commandExecution", now.Add(time.Millisecond).UnixMilli(), cli); err != nil {
		t.Fatal(err)
	}
	history.Close()
	if count, err := poller.Poll(context.Background()); err != nil || count != 2 {
		t.Fatalf("capture: %d, %v", count, err)
	}
	var imported []sage.Event
	result, err := sage.ImportSpool(root, 10, func(event sage.Event) (bool, error) {
		imported = append(imported, event)
		return true, nil
	})
	if err != nil || result.Imported != 2 || len(imported) != 2 {
		t.Fatalf("import=%+v events=%+v err=%v", result, imported, err)
	}
	commands := map[string]bool{}
	for _, event := range imported {
		commands[event.Command] = true
		if event.Harness != "codex-history" || event.Command == "git log" || event.Command == "git log --oneline" {
			t.Fatalf("unexpected event=%+v", event)
		}
	}
	if !commands["rg -c -i -n"] || !commands["git status --short"] {
		t.Fatalf("privacy-minimized commands=%v", commands)
	}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("repeat: %d, %v", count, err)
	}
}

func TestReferencePauseDoesNotReadOrAdvanceHistory(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	history.Close()
	now := time.UnixMilli(6_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if _, err := poller.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, stateFilename)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sage.SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	poller.CodexHome = filepath.Join(home, "missing")
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("paused poll: %d, %v", count, err)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("state advanced while paused: before=%s after=%s err=%v", before, after, err)
	}
}

func TestReferenceResetExcludesCommandsFromThePausedPeriod(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	poller := Poller{Root: root, CodexHome: home, Now: time.Now}
	if _, err := poller.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := sage.SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	pausedAt, err := sage.CaptureCutoff(root)
	if err != nil {
		t.Fatal(err)
	}
	paused := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"git status --short"}]}`
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?)`, "ide-thread", "paused", 1, "commandExecution", pausedAt+1, paused); err != nil {
		t.Fatal(err)
	}
	history.Close()
	for time.Now().UnixMilli() <= pausedAt+1 {
		time.Sleep(time.Millisecond)
	}
	if _, err := sage.SetPaused(root, false); err != nil {
		t.Fatal(err)
	}
	if count, err := poller.Poll(context.Background()); err != nil || count != 0 {
		t.Fatalf("paused-period command captured: %d, %v", count, err)
	}
}

func TestReferenceFailureDoesNotAdvanceCheckpoint(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	index, history := createHistoryDatabases(t, home)
	index.Close()
	now := time.UnixMilli(7_000_000).UTC()
	poller := Poller{Root: root, CodexHome: home, Now: func() time.Time { return now }}
	if _, err := poller.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	completed := `{"type":"commandExecution","status":"completed","exitCode":0,"commandActions":[{"command":"git status --short"}]}`
	if _, err := history.Exec(`INSERT INTO thread_items VALUES (?,?,?,?,?,?)`, "ide-thread", "retry", 1, "commandExecution", now.Add(time.Millisecond).UnixMilli(), completed); err != nil {
		t.Fatal(err)
	}
	history.Close()
	statePath := filepath.Join(root, stateFilename)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	poller.WriteEvents = func(string, []sage.Event) (sage.CaptureResult, error) {
		return sage.CaptureResult{}, errors.New("disk full")
	}
	if count, err := poller.Poll(context.Background()); err == nil || count != 0 {
		t.Fatalf("enqueue failure: %d, %v", count, err)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || string(after) != string(before) {
		t.Fatalf("checkpoint advanced: before=%s after=%s err=%v", before, after, err)
	}
	poller.WriteEvents = sage.WriteEvents
	if count, err := poller.Poll(context.Background()); err != nil || count != 1 {
		t.Fatalf("retry: %d, %v", count, err)
	}
}
