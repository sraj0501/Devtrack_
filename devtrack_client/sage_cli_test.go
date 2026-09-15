package main

import (
	"bytes"
	"encoding/json"
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
