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

func TestCursorInstallIsPortableAdditiveAndByteStable(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hooks.json")
	original := `{"version":1,"foreign_empty":{},"hooks":{"afterShellExecution":[{"type":"command","command":"foreign-shell"}],"postToolUse":[{"type":"command","command":"foreign-tool","matcher":"Read"}],"sessionStart":[{"type":"command","command":"foreign-session"}]}}`
	if err := os.WriteFile(path, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	installer := CursorInstaller{CursorHome: home, Executable: `C:\Program Files\DevTrack\devtrack.exe`}
	first, err := installer.Install()
	if err != nil || !first.Installed || !first.Changed || first.Mode != "cursor-hook" || first.Outcome != "updated" {
		t.Fatalf("install=%+v err=%v", first, err)
	}
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw1)
	for _, expected := range []string{OwnerMarker, `"version": 1`, `"afterShellExecution"`, `"postToolUse"`,
		`"matcher": "Shell|Write"`, `"timeout": 10`, `C:\\Program Files\\DevTrack\\devtrack.exe`, `sage hook cursor`,
		"foreign-shell", "foreign-tool", "foreign-session"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q: %s", expected, raw1)
		}
	}
	if strings.Count(text, OwnerMarker) != 2 || strings.Count(text, `"sessionStart"`) != 1 {
		t.Fatalf("capture ownership or lifecycle changed: %s", raw1)
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
		t.Fatalf("reinstall=%+v err=%v", second, err)
	}
	raw2, _ := os.ReadFile(path)
	if !bytes.Equal(raw1, raw2) {
		t.Fatal("reinstall rewrote Cursor hook file")
	}
	status, err := installer.Status()
	if err != nil || !status.Installed || status.Mode != "cursor-hook" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	removed, err := installer.Uninstall()
	if err != nil || !removed.Changed || removed.Outcome != "updated" {
		t.Fatalf("uninstall=%+v err=%v", removed, err)
	}
	cleaned, _ := os.ReadFile(path)
	if strings.Contains(string(cleaned), OwnerMarker) || !strings.Contains(string(cleaned), "foreign-shell") ||
		!strings.Contains(string(cleaned), "foreign-tool") || !strings.Contains(string(cleaned), "foreign-session") ||
		!strings.Contains(string(cleaned), `"foreign_empty": {}`) {
		t.Fatalf("uninstall changed foreign data: %s", cleaned)
	}
}

func TestCursorInstallHandlesMissingInvalidAndLegacyFiles(t *testing.T) {
	t.Run("missing install", func(t *testing.T) {
		home := t.TempDir()
		result, err := (CursorInstaller{CursorHome: home, Executable: "devtrack"}).Install()
		if err != nil || !result.Installed || !result.Changed || result.Outcome != "created" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "hooks.json")); err != nil {
			t.Fatalf("install did not create the hook file: %v", err)
		}
	})
	t.Run("missing uninstall", func(t *testing.T) {
		home := t.TempDir()
		result, err := (CursorInstaller{CursorHome: home}).Uninstall()
		if err != nil || result.Changed || result.Outcome != "missing" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "hooks.json")); !os.IsNotExist(err) {
			t.Fatalf("uninstall created a file: %v", err)
		}
	})
	for name, original := range map[string]string{
		"invalid hooks shape": `{"version":1,"hooks":"foreign"}`,
		"invalid version":     `{"version":2,"hooks":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "hooks.json")
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (CursorInstaller{CursorHome: home, Executable: "devtrack"}).Install(); err == nil {
				t.Fatal("invalid hook file was accepted")
			}
			raw, _ := os.ReadFile(path)
			if string(raw) != original {
				t.Fatal("invalid hook file was overwritten")
			}
		})
	}
	t.Run("legacy entries replaced", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "hooks.json")
		legacy := `{"version":1,"hooks":{"sessionStart":[{"command":"/old/ai_sessions_skills/hook-session.sh"}],"afterShellExecution":[{"command":"/old/ai_sessions_skills/hook-capture.sh"}]}}`
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (CursorInstaller{CursorHome: home, Executable: "devtrack"}).Install(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "ai_sessions_skills") || strings.Count(string(raw), OwnerMarker) != 2 || !strings.Contains(string(raw), `"sessionStart": []`) {
			t.Fatalf("legacy entries were not replaced: %s", raw)
		}
	})
}

func TestCursorPathUsesReferenceHomeRelativeLayout(t *testing.T) {
	home := t.TempDir()
	path, err := (CursorInstaller{CursorHome: filepath.Join(home, ".cursor")}).hooksPath()
	if err != nil || path != filepath.Join(home, ".cursor", "hooks.json") {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestNormalizeCursorEventsMinimizesSupportedPayloads(t *testing.T) {
	when := time.Date(2026, 10, 15, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		name, payload, command, signature, tool string
		success                                 bool
	}{
		{"shell", `{"hook_event_name":"afterShellExecution","conversation_id":"private-session","cwd":"C:/private/project","command":"pnpm dlx prisma migrate dev","exit_code":0,"output":"PRIVATE_OUTPUT"}`, "pnpm dlx", "pnpm dlx", "shell", true},
		{"failed shell", `{"eventName":"afterShellExecution","conversationId":"private-session","generationId":"call-2","command":"go test ./private/package -race","exitCode":1,"stderr":"PRIVATE_ERROR"}`, "go test -race", "go test -race", "shell", false},
		{"write", `{"hook_event_name":"postToolUse","session_id":"private-session","tool_use_id":"call-3","tool_name":"Write","tool_input":{"path":"C:/private/file","contents":"PRIVATE_CONTENT"}}`, "cat", "agent-tool write", "agent-tool", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := NormalizeCursorEvents([]byte(tc.payload), when)
			if len(events) != 1 {
				t.Fatalf("events=%+v", events)
			}
			event := events[0]
			if event.Command != tc.command || event.Signature != tc.signature || event.Tool != tc.tool ||
				event.Harness != "cursor" || event.Success == nil || *event.Success != tc.success {
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

func TestNormalizeCursorEventsDeduplicatesHookShapesAndFailsOpen(t *testing.T) {
	when := time.Now()
	afterShell := `{"hook_event_name":"afterShellExecution","cwd":"C:/project","command":"git status","exit_code":0}`
	postTool := `{"hook_event_name":"postToolUse","cwd":"C:/project","tool_name":"Shell","tool_input":{"command":"git status"},"tool_response":{"success":true}}`
	first := NormalizeCursorEvents([]byte(afterShell), when)
	second := NormalizeCursorEvents([]byte(postTool), when.Add(time.Minute))
	if len(first) != 1 || len(second) != 1 || first[0].DeliveryKey() != second[0].DeliveryKey() {
		t.Fatalf("fallback identity is not deterministic: %+v %+v", first, second)
	}
	for _, raw := range [][]byte{
		nil, []byte(`{`), bytes.Repeat([]byte("x"), MaxHarnessHookBytes+1),
		[]byte(`{"hook_event_name":"sessionStart","command":"git status"}`),
		[]byte(`{"hook_event_name":"postToolUse","tool_name":"Unknown","tool_input":{"command":"git status"}}`),
		[]byte(`{"hook_event_name":"postToolUse","tool_name":"Shell","tool_input":{}}`),
	} {
		if events := NormalizeCursorEvents(raw, when); len(events) != 0 {
			t.Fatalf("unsafe input produced events: %+v", events)
		}
	}
}
