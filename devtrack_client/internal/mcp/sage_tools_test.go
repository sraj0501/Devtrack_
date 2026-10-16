package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/harness"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func writeSageTestEntry(t *testing.T, writer knowledge.Writer, topic, signature, needle string) {
	t.Helper()
	_, err := writer.Write(topic, signature, distill.Draft{
		Title:    "Review staged work",
		Section:  "Review",
		Commands: []string{"git diff --staged"},
		What:     "Inspect " + needle,
		Why:      "Catch regressions before committing",
		Example:  "Before opening a review",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegisterSageToolsPreservesExistingSurface(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()

	srv := New("test-v0")
	RegisterDevTrackTools(srv, database)
	RegisterSageTools(srv, database, t.TempDir(), knowledge.Writer{Root: t.TempDir()}, nil)

	tools := srv.toolList()
	if len(tools) != 10 {
		t.Fatalf("expected 10 tools, got %d", len(tools))
	}
	wantSage := map[string]bool{
		"sage_search": false, "sage_topics": false, "sage_get_entry": false, "sage_status": false,
	}
	for _, tool := range tools {
		name := tool["name"].(string)
		if _, ok := wantSage[name]; !ok {
			continue
		}
		wantSage[name] = true
		annotations := tool["annotations"].(map[string]any)
		for key, want := range map[string]bool{
			"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false,
		} {
			if got, ok := annotations[key].(bool); !ok || got != want {
				t.Errorf("%s annotation %s = %#v, want %t", name, key, annotations[key], want)
			}
		}
		schema := tool["inputSchema"].(map[string]any)
		if schema["additionalProperties"] != false {
			t.Errorf("%s schema permits undeclared arguments", name)
		}
	}
	for name, found := range wantSage {
		if !found {
			t.Errorf("missing tool %s", name)
		}
	}
}

func TestSageKnowledgeToolsReconcileCurrentMarkdown(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()
	writer := knowledge.Writer{Root: t.TempDir()}
	writeSageTestEntry(t, writer, "git", "abcdef01", "originalneedle")

	search := makeSageSearch(database, writer)
	result, err := search(context.Background(), map[string]any{"query": "git originalneedle", "topic": "git.md", "limit": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	entries := result.(map[string]any)["entries"].([]map[string]any)
	if len(entries) != 1 || !strings.Contains(entries[0]["body"].(string), "originalneedle") {
		t.Fatalf("search did not return the complete entry: %#v", result)
	}
	line := entries[0]["line"].(int)

	topics, err := makeSageTopics(database, writer)(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	topicRows := topics.(map[string]any)["topics"].([]map[string]any)
	if len(topicRows) != 1 || topicRows[0]["name"] != "git" || topicRows[0]["entries"] != 1 {
		t.Fatalf("unexpected topics: %#v", topics)
	}

	get := makeSageGetEntry(database, writer)
	exact, err := get(context.Background(), map[string]any{"topic": "git", "line": line})
	if err != nil || !strings.Contains(exact.(map[string]any)["entry"].(map[string]any)["body"].(string), "originalneedle") {
		t.Fatalf("exact entry: %#v, %v", exact, err)
	}

	oldPath := filepath.Join(writer.Root, "git.md")
	newPath := filepath.Join(writer.Root, "manual.md")
	raw, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte(strings.ReplaceAll(string(raw), "originalneedle", "editedneedle")), 0600); err != nil {
		t.Fatal(err)
	}
	if edited, err := get(context.Background(), map[string]any{"topic": "git", "line": line}); err != nil || !strings.Contains(edited.(map[string]any)["entry"].(map[string]any)["body"].(string), "editedneedle") {
		t.Fatalf("edited entry was not refreshed: %#v, %v", edited, err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if _, err := get(context.Background(), map[string]any{"topic": "git", "line": line}); err == nil || err.Error() != "sage entry not found" {
		t.Fatalf("old moved identifier remained available: %v", err)
	}
	moved, err := search(context.Background(), map[string]any{"query": "editedneedle", "topic": "manual"})
	if err != nil {
		t.Fatal(err)
	}
	movedEntries := moved.(map[string]any)["entries"].([]map[string]any)
	if len(movedEntries) != 1 || movedEntries[0]["topic"] != "manual" {
		t.Fatalf("moved entry was not reconciled: %#v", moved)
	}
	if _, err := get(context.Background(), map[string]any{"topic": "manual", "line": movedEntries[0]["line"]}); err != nil {
		t.Fatalf("moved entry unavailable by current identifier: %v", err)
	}
	if err := os.Remove(newPath); err != nil {
		t.Fatal(err)
	}
	deleted, err := search(context.Background(), map[string]any{"query": "editedneedle"})
	if err != nil || deleted.(map[string]any)["count"] != 0 {
		t.Fatalf("deleted entry remained searchable: %#v, %v", deleted, err)
	}
}

func TestSageStatusIsBoundedAndPrivacySafe(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spool", "pending"), 0700); err != nil {
		t.Fatal(err)
	}
	const rawCanary = "raw-command-and-prompt-canary"
	if err := os.WriteFile(filepath.Join(root, "spool", "pending", "event.json"), []byte(rawCanary), 0600); err != nil {
		t.Fatal(err)
	}
	adapters := func() ([]harness.Descriptor, error) {
		return []harness.Descriptor{{ID: "codex", DisplayName: "Codex", BuiltIn: true, Installed: true, Mode: "codex-hook"}}, nil
	}
	result, err := makeSageStatus(database, root, adapters)(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, rawCanary) || strings.Contains(text, "command\"") || strings.Contains(text, "prompt\"") || strings.Contains(text, "reasoning\"") {
		t.Fatalf("status exposed raw activity: %s", text)
	}
	if !strings.Contains(text, `"backlog":1`) || !strings.Contains(text, `"mode":"codex-hook"`) {
		t.Fatalf("status omitted bounded diagnostics: %s", text)
	}

	if _, err := makeSageStatus(database, root, func() ([]harness.Descriptor, error) {
		return nil, errors.New(rawCanary)
	})(context.Background(), map[string]any{}); err == nil || strings.Contains(err.Error(), rawCanary) {
		t.Fatalf("adapter error was not safely bounded: %v", err)
	}
}

func TestSageToolArgumentsFailSafely(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()
	writer := knowledge.Writer{Root: t.TempDir()}
	tests := []struct {
		name    string
		handler func(context.Context, map[string]any) (any, error)
		args    map[string]any
	}{
		{"missing query", makeSageSearch(database, writer), map[string]any{}},
		{"wrong query type", makeSageSearch(database, writer), map[string]any{"query": true}},
		{"oversized query", makeSageSearch(database, writer), map[string]any{"query": strings.Repeat("x", 4097)}},
		{"fractional limit", makeSageSearch(database, writer), map[string]any{"query": "git", "limit": 1.5}},
		{"large limit", makeSageSearch(database, writer), map[string]any{"query": "git", "limit": 101}},
		{"unknown search argument", makeSageSearch(database, writer), map[string]any{"query": "git", "raw": true}},
		{"topics argument", makeSageTopics(database, writer), map[string]any{"raw": true}},
		{"missing line", makeSageGetEntry(database, writer), map[string]any{"topic": "git"}},
		{"wrong line type", makeSageGetEntry(database, writer), map[string]any{"topic": "git", "line": "1"}},
		{"empty topic", makeSageGetEntry(database, writer), map[string]any{"topic": " ", "line": 1}},
		{"missing entry", makeSageGetEntry(database, writer), map[string]any{"topic": "git", "line": 1}},
		{"status argument", makeSageStatus(database, t.TempDir(), nil), map[string]any{"raw": true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.handler(context.Background(), tc.args); err == nil || len(err.Error()) > 256 {
				t.Fatalf("expected bounded error, got %v", err)
			}
		})
	}
}

func TestSageToolsRespectCancelledContext(t *testing.T) {
	database, cleanup := openTestDB(t)
	defer cleanup()
	writer := knowledge.Writer{Root: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name    string
		handler func(context.Context, map[string]any) (any, error)
		args    map[string]any
	}{
		{"search", makeSageSearch(database, writer), map[string]any{"query": "git"}},
		{"topics", makeSageTopics(database, writer), map[string]any{}},
		{"get", makeSageGetEntry(database, writer), map[string]any{"topic": "git", "line": 1}},
		{"status", makeSageStatus(database, t.TempDir(), nil), map[string]any{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.handler(ctx, tc.args); !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context cancellation, got %v", err)
			}
		})
	}
}
