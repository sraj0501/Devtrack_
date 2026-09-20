package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestRouteSageCompatibility(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		sub    string
		rest   string
		legacy bool
	}{
		{"bare legacy chat", nil, "interactive", "", true},
		{"old ask", []string{"ask", "why"}, "ask", "why", true},
		{"old do", []string{"do", "task"}, "do", "task", true},
		{"old pr", []string{"pr"}, "pr", "", true},
		{"old interactive", []string{"interactive"}, "interactive", "", true},
		{"explicit git", []string{"git", "ask", "why"}, "ask", "why", false},
		{"explicit git chat", []string{"git"}, "interactive", "", false},
		{"new status", []string{"status"}, "status", "", false},
		{"new search", []string{"search", "git"}, "search", "git", false},
		{"install hooks", []string{"install-hooks"}, "install-hooks", "", false},
		{"remove hooks", []string{"uninstall-hooks"}, "uninstall-hooks", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub, rest, legacy := routeSage(tc.args)
			if sub != tc.sub || legacy != tc.legacy || len(rest) > 1 || (len(rest) == 1 && rest[0] != tc.rest) || (len(rest) == 0 && tc.rest != "") {
				t.Fatalf("routeSage(%q) = %q, %q, %t", tc.args, sub, rest, legacy)
			}
		})
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
