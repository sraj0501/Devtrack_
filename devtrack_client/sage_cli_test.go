package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
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
