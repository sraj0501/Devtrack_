package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestClaudeInstallIsAdditiveIdempotentAndPureSubtraction(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "settings.json")
	original := `{"theme":"dark","foreign_empty":{},"hooks":{"PostToolUse":[{"matcher":"Other","hooks":[{"type":"command","command":"foreign-tool"}]}],"SessionStart":[{"hooks":[{"command":"foreign-session"}]}]}}`
	if err := os.WriteFile(path, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	installer := ClaudeInstaller{ClaudeHome: home, Executable: `C:\Program Files\DevTrack\devtrack.exe`}
	first, err := installer.Install()
	if err != nil || !first.Installed || !first.Changed || first.Mode != "claude-hook" || first.Outcome != "updated" {
		t.Fatalf("install: %+v err=%v", first, err)
	}
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw1)
	if strings.Count(text, OwnerMarker) != 2 || !strings.Contains(text, claudeMatcher) ||
		!strings.Contains(text, `"matcher": "Bash"`) || !strings.Contains(text, `"theme": "dark"`) ||
		!strings.Contains(text, "foreign-tool") || !strings.Contains(text, "foreign-session") ||
		!strings.Contains(text, `\"C:\\Program Files\\DevTrack\\devtrack.exe\" sage hook claude`) {
		t.Fatalf("bad Claude merge: %s", raw1)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	second, err := installer.Install()
	if err != nil || second.Changed || second.Outcome != "unchanged" {
		t.Fatalf("reinstall: %+v err=%v", second, err)
	}
	raw2, _ := os.ReadFile(path)
	if !bytes.Equal(raw1, raw2) {
		t.Fatal("idempotent install rewrote Claude settings")
	}
	removed, err := installer.Uninstall()
	if err != nil || !removed.Changed || removed.Installed || removed.Outcome != "updated" {
		t.Fatalf("uninstall: %+v err=%v", removed, err)
	}
	cleaned, _ := os.ReadFile(path)
	if strings.Contains(string(cleaned), OwnerMarker) || !strings.Contains(string(cleaned), "foreign-tool") ||
		!strings.Contains(string(cleaned), "foreign-session") || !strings.Contains(string(cleaned), `"theme": "dark"`) ||
		!strings.Contains(string(cleaned), `"foreign_empty": {}`) {
		t.Fatalf("bad Claude subtraction: %s", cleaned)
	}
}

func TestClaudeInstallHandlesMissingInvalidAndLegacySettings(t *testing.T) {
	t.Run("missing uninstall", func(t *testing.T) {
		home := t.TempDir()
		result, err := (ClaudeInstaller{ClaudeHome: home}).Uninstall()
		if err != nil || result.Changed || result.Outcome != "missing" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "settings.json")); !os.IsNotExist(err) {
			t.Fatalf("uninstall created settings: %v", err)
		}
	})
	t.Run("invalid is preserved", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "settings.json")
		if err := os.WriteFile(path, []byte(`{invalid`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (ClaudeInstaller{ClaudeHome: home, Executable: "devtrack"}).Install(); err == nil {
			t.Fatal("invalid settings were accepted")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != `{invalid` {
			t.Fatal("invalid settings were overwritten")
		}
	})
	t.Run("invalid hooks shape is preserved", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "settings.json")
		original := `{"hooks":"foreign-value","theme":"dark"}`
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (ClaudeInstaller{ClaudeHome: home, Executable: "devtrack"}).Install(); err == nil {
			t.Fatal("invalid hooks shape was accepted")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != original {
			t.Fatal("foreign hooks shape was overwritten")
		}
	})
	t.Run("legacy hook is replaced", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "settings.json")
		legacy := `{"hooks":{"PostToolUse":[{"hooks":[{"command":"C:\\old\\ai_sessions_skills\\hook-capture.cmd claude"}]}]}}`
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (ClaudeInstaller{ClaudeHome: home, Executable: "devtrack"}).Install(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "ai_sessions_skills") || strings.Count(string(raw), OwnerMarker) != 2 {
			t.Fatalf("legacy hook not replaced: %s", raw)
		}
	})
}

func TestNormalizeClaudeEventsCoversShellSearchEditAndFailure(t *testing.T) {
	when := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name, payload, command, signature, tool string
		success                                 bool
	}{
		{"shell", `{"session_id":"private-session","cwd":"C:/private/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"tool-1","tool_input":{"command":"git status --short"},"tool_response":"PRIVATE_OUTPUT"}`, "git status --short", "git status --short", "shell", true},
		{"failure", `{"session_id":"private-session","hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_use_id":"tool-2","tool_input":{"command":"go test ./private/package -race"},"error":"PRIVATE_ERROR"}`, "go test -race", "go test -race", "shell", false},
		{"grep", `{"session_id":"private-session","hook_event_name":"PostToolUse","tool_name":"Grep","tool_use_id":"tool-3","tool_input":{"pattern":"PRIVATE_PATTERN","glob":"PRIVATE_GLOB","output_mode":"count"}}`, "rg --count --glob --regexp", "rg --count --glob --regexp", "agent-tool", true},
		{"edit", `{"session_id":"private-session","hook_event_name":"PostToolUse","tool_name":"Edit","tool_use_id":"tool-4","tool_input":{"file_path":"C:/private/file","old_string":"PRIVATE_OLD","new_string":"PRIVATE_NEW"}}`, "sed -i", "agent-tool edit", "agent-tool", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := NormalizeClaudeEvents([]byte(tc.payload), when)
			if len(events) != 1 {
				t.Fatalf("events=%+v", events)
			}
			event := events[0]
			if event.Command != tc.command || event.Signature != tc.signature || event.Tool != tc.tool ||
				event.Harness != "claude" || event.Success == nil || *event.Success != tc.success {
				t.Fatalf("event=%+v", event)
			}
			encoded, _ := json.Marshal(event)
			for _, private := range []string{"private-session", "C:/private", "PRIVATE_"} {
				if bytes.Contains(bytes.ToUpper(encoded), []byte(strings.ToUpper(private))) {
					t.Fatalf("private payload leaked: %s", encoded)
				}
			}
			if _, err := sage.DecodeEvent(encoded); err != nil {
				t.Fatalf("invalid Sage event: %v", err)
			}
		})
	}
}

func TestNormalizeClaudeEventsFailsOpenAndBoundsInput(t *testing.T) {
	when := time.Now()
	valid := `{"session_id":"s","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"t","tool_input":{"command":"git status"}}`
	cases := [][]byte{
		nil,
		[]byte(`{`),
		bytes.Repeat([]byte("x"), MaxHarnessHookBytes+1),
		[]byte(strings.Replace(valid, "PostToolUse", "PreToolUse", 1)),
		[]byte(strings.Replace(valid, `"tool_use_id":"t"`, `"tool_use_id":""`, 1)),
		[]byte(strings.Replace(valid, `"tool_name":"Bash"`, `"tool_name":"Unknown"`, 1)),
	}
	for _, raw := range cases {
		if events := NormalizeClaudeEvents(raw, when); len(events) != 0 {
			t.Fatalf("unexpected events: %+v", events)
		}
	}
}
