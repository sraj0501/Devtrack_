package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	sageknowledge "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func TestRouteSageRejectsLegacyAndUnknownCommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		sub  string
		rest []string
	}{
		{[]string{"status"}, "status", nil},
		{[]string{"search", "git"}, "search", []string{"git"}},
		{[]string{"routes"}, "routes", nil},
		{[]string{"merge", "grep", "shell"}, "merge", []string{"grep", "shell"}},
		{[]string{"route", "git", "version-control"}, "route", []string{"git", "version-control"}},
		{[]string{"harness", "list"}, "harness", []string{"list"}},
		{[]string{"hook", "codex"}, "hook", []string{"codex"}},
	} {
		sub, rest, err := routeSage(tc.args)
		if err != nil || sub != tc.sub || len(rest) != len(tc.rest) {
			t.Fatalf("routeSage(%q) = %q, %q, %v", tc.args, sub, rest, err)
		}
		for i := range rest {
			if rest[i] != tc.rest[i] {
				t.Fatalf("routeSage(%q) rest = %q", tc.args, rest)
			}
		}
	}

	for _, args := range [][]string{
		nil,
		{"ask", "why"},
		{"do", "task"},
		{"pr"},
		{"interactive"},
		{"git", "ask", "why"},
		{"what", "changed"},
		{"statsu"},
	} {
		if _, _, err := routeSage(args); err == nil {
			t.Fatalf("routeSage(%q) accepted a removed or unknown command", args)
		}
	}
}

func TestSageRoutesCLIUsesConfiguredKnowledgeAndValidatesBeforeOpeningDatabase(t *testing.T) {
	for _, args := range [][]string{nil, {"source"}, {"source", ""}, {"a", "b", "c"}} {
		var output bytes.Buffer
		if err := runSageRoutes("merge", args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("invalid merge accepted: %v %v", args, err)
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", root)
	w := sageknowledge.Writer{Root: root}
	if err := w.RememberRoute("git", "version-control"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runSageRoutes("routes", nil, &output); err != nil {
		t.Fatal(err)
	}
	var routes []sageknowledge.Route
	if err := json.Unmarshal(output.Bytes(), &routes); err != nil || len(routes) != 1 || routes[0].Filename != "version-control.md" || routes[0].Source != "remembered" {
		t.Fatalf("%s %v", output.String(), err)
	}
	for _, tc := range []struct {
		sub  string
		args []string
	}{
		{"routes", []string{"extra"}}, {"route", nil}, {"route", []string{"git"}},
		{"route", []string{"git", "topic", "extra"}}, {"route", []string{"../bad", "topic"}},
	} {
		output.Reset()
		if err := runSageRoutes(tc.sub, tc.args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("accepted %s %v", tc.sub, tc.args)
		}
	}
}

func TestPackagedCodexHookPathWritesOneSilentSpoolEvent(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	payload := `{"session_id":"thread","cwd":"C:/private/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"call","tool_input":{"command":"git status --short"},"tool_response":"CANARY"}`
	var output bytes.Buffer
	if err := runSageHook([]string{"codex", "--devtrack-sage-hook"}, bytes.NewBufferString(payload)); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("hook wrote output: %q", output.String())
	}
	entries, err := os.ReadDir(filepath.Join(dataHome, "devtrack", "sage", "spool", "pending"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("spool files=%d err=%v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(dataHome, "devtrack", "sage", "spool", "pending", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("CANARY")) || bytes.Contains(raw, []byte("C:/private")) {
		t.Fatalf("private payload leaked: %s", raw)
	}
}

func TestSelectiveHarnessCLIListsInstallsAndRemovesCodex(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	var output bytes.Buffer
	if err := runSageHarness([]string{"list"}, &output); err != nil {
		t.Fatal(err)
	}
	var listed []struct {
		ID        string `json:"id"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal(output.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0].ID != "codex" || listed[0].Installed {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	output.Reset()
	if err := runSageHarness([]string{"install", "codex"}, &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runSageHarness([]string{"list"}, &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &listed); err != nil || !listed[0].Installed {
		t.Fatalf("installed list=%+v err=%v", listed, err)
	}
	if err := runSageHarness([]string{"uninstall", "codex"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := runSageHarness([]string{"install", "missing"}, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown harness must fail")
	}
}

func TestSageStateCommandsEmitJSONWithoutStartingCapture(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, command := range []string{"status", "pause", "doctor", "resume"} {
		var output bytes.Buffer
		if err := writeSageState(command, nil, &output); err != nil {
			t.Fatal(err)
		}
		var state sage.State
		if err := json.NewDecoder(&output).Decode(&state); err != nil {
			t.Fatalf("%s JSON: %v", command, err)
		}
		if state.Capture != "not_installed" || state.Ready || state.Paused != (command == "pause" || command == "doctor") {
			t.Fatalf("%s reported %+v", command, state)
		}
	}
	if err := writeSageState("status", []string{"unexpected"}, &bytes.Buffer{}); err == nil {
		t.Fatal("invalid arguments must fail")
	}
}

type fakeSageKnowledgeStore struct {
	query string
	topic string
}

func (f *fakeSageKnowledgeStore) SearchSageKnowledge(query, topic string, _ int) ([]sageknowledge.Entry, error) {
	f.query, f.topic = query, topic
	return []sageknowledge.Entry{{
		Signature: "git status", Topic: "git", Command: "git status --short",
		UseCount: 1, SuccessCount: 1, LastSeen: time.Unix(10, 0),
	}}, nil
}

func (f *fakeSageKnowledgeStore) ListSageTopics() ([]sageknowledge.Topic, error) {
	return nil, nil
}

func TestSageSearchParsesTopicAndRendersKnowledge(t *testing.T) {
	store := &fakeSageKnowledgeStore{}
	var output bytes.Buffer
	if err := writeSageSearch(store, []string{"status", "command", "--topic", "git"}, &output); err != nil {
		t.Fatal(err)
	}
	if store.query != "status command" || store.topic != "git" {
		t.Fatalf("query=%q topic=%q", store.query, store.topic)
	}
	if !bytes.Contains(output.Bytes(), []byte("## git status")) {
		t.Fatalf("output=%q", output.String())
	}
	if err := writeSageSearch(store, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("missing query must fail")
	}
}

func TestSageSearchRendersCompleteCurrentEntry(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	w := sageknowledge.Writer{Root: t.TempDir()}
	draft := distill.Draft{Title: "Review", Section: "Committing", Commands: []string{"git diff --staged"}, What: "Inspect staged changes", Why: "Catch subtle regressions", Example: "Before commit", Notes: "Preserve this complete note"}
	if _, err := w.Write("git", "abcdef01", draft); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteSkipped("abcdef02"); err != nil {
		t.Fatal(err)
	}
	store := sageEntryStore{database: database, writer: w}
	var out bytes.Buffer
	if err := writeSageSearch(store, []string{"subtle", "--topic", "git"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"git.md:", "### Review", "git diff --staged", "Catch subtle regressions", "Preserve this complete note"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Skipped action") {
		t.Fatal("skip appeared in search")
	}
	for _, args := range [][]string{{" "}, {"git", "--topic", ""}, {"git", "--topic", "git", "--topic", "other"}} {
		if err := writeSageSearch(store, args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
