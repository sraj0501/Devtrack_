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

func TestOpenCodeInstallIsPortableByteStableAndPureSubtraction(t *testing.T) {
	home := t.TempDir()
	installer := OpenCodeInstaller{OpenCodeHome: home, Executable: `C:\Program Files\Dev'Track\devtrack.exe`}
	first, err := installer.Install()
	if err != nil || !first.Installed || !first.Changed || first.Mode != "opencode-plugin" || first.Outcome != "created" {
		t.Fatalf("install=%+v err=%v", first, err)
	}
	path := filepath.Join(home, "plugin", "ai-session-skills.js")
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw1)
	for _, expected := range []string{OwnerMarker, `"tool.execute.before"`, `"tool.execute.after"`,
		`["sage", "hook", "opencode", "--devtrack-sage-hook"]`, `C:\\Program Files\\Dev'Track\\devtrack.exe`,
		`windowsHide: true`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q: %s", expected, raw1)
		}
	}
	for _, forbidden := range []string{"session.idle", "hook-session", "ai_sessions_skills"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unexpected lifecycle or legacy marker %q: %s", forbidden, raw1)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	second, err := installer.Install()
	if err != nil || second.Changed || second.Outcome != "unchanged" {
		t.Fatalf("reinstall=%+v err=%v", second, err)
	}
	raw2, _ := os.ReadFile(path)
	if !bytes.Equal(raw1, raw2) {
		t.Fatal("reinstall rewrote OpenCode plugin")
	}
	status, err := installer.Status()
	if err != nil || !status.Installed || status.Mode != "opencode-plugin" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	removed, err := installer.Uninstall()
	if err != nil || !removed.Changed || removed.Outcome != "updated" {
		t.Fatalf("uninstall=%+v err=%v", removed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("owned plugin remained: %v", err)
	}
}

func TestOpenCodeInstallPreservesUnknownFilesAndReplacesLegacy(t *testing.T) {
	t.Run("unknown file", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "plugin", "ai-session-skills.js")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		original := []byte("export default function foreignPlugin() {}\n")
		if err := os.WriteFile(path, original, 0640); err != nil {
			t.Fatal(err)
		}
		installer := OpenCodeInstaller{OpenCodeHome: home, Executable: "devtrack"}
		if _, err := installer.Install(); err == nil {
			t.Fatal("unowned plugin was overwritten")
		}
		result, err := installer.Uninstall()
		if err != nil || result.Changed || result.Outcome != "unchanged" {
			t.Fatalf("uninstall=%+v err=%v", result, err)
		}
		raw, _ := os.ReadFile(path)
		if !bytes.Equal(raw, original) {
			t.Fatal("unowned plugin changed")
		}
	})
	t.Run("legacy file", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "plugin", "ai-session-skills.js")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("const CAPTURE='/old/ai_sessions_skills/hook-capture.sh';\n"), 0640); err != nil {
			t.Fatal(err)
		}
		installer := OpenCodeInstaller{OpenCodeHome: home, Executable: "devtrack"}
		result, err := installer.Install()
		if err != nil || !result.Changed || result.Outcome != "updated" {
			t.Fatalf("install=%+v err=%v", result, err)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "ai_sessions_skills") || !strings.Contains(string(raw), OwnerMarker) {
			t.Fatalf("legacy plugin was not replaced: %s", raw)
		}
		info, _ := os.Stat(path)
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
			t.Fatalf("mode=%o", info.Mode().Perm())
		}
	})
	t.Run("missing uninstall", func(t *testing.T) {
		home := t.TempDir()
		result, err := (OpenCodeInstaller{OpenCodeHome: home}).Uninstall()
		if err != nil || result.Changed || result.Outcome != "missing" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "plugin", "ai-session-skills.js")); !os.IsNotExist(err) {
			t.Fatalf("uninstall created a plugin: %v", err)
		}
	})
}

func TestOpenCodePluginPathUsesReferencePlatformLayout(t *testing.T) {
	home := t.TempDir()
	appdata := filepath.Join(home, "AppData", "Roaming")
	existing := filepath.Join(home, ".config", "opencode")
	installer := OpenCodeInstaller{GOOS: "windows", Home: home, Env: map[string]string{"APPDATA": appdata},
		Exists: func(path string) bool { return path == existing }}
	path, err := installer.pluginPath()
	if err != nil || path != filepath.Join(existing, "plugin", "ai-session-skills.js") {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestNormalizeOpenCodeEventsMinimizesSupportedPayloads(t *testing.T) {
	when := time.Date(2026, 10, 15, 13, 0, 0, 0, time.UTC)
	cases := []struct {
		name, payload, command, signature, tool string
		success                                 bool
	}{
		{"shell", `{"hook_event_name":"PostToolUse","session_id":"private-session","cwd":"C:/private/project","tool_name":"bash","tool_input":{"command":"bun test --coverage"},"tool_response":{"success":true,"output":"PRIVATE_OUTPUT"}}`, "bun test --coverage", "bun test --coverage", "shell", true},
		{"failed shell", `{"session_id":"private-session","call_id":"call-2","tool_name":"bash","tool_input":{"command":"go test ./private/package -race"},"tool_response":{"success":false,"error":"PRIVATE_ERROR"}}`, "go test -race", "go test -race", "shell", false},
		{"search", `{"session_id":"private-session","call_id":"call-3","tool_name":"grep","tool_input":{"pattern":"PRIVATE_PATTERN","include":"PRIVATE_GLOB","outputMode":"count"},"tool_response":{"success":true}}`, "rg --count --glob --regexp", "rg --count --glob --regexp", "agent-tool", true},
		{"edit", `{"session_id":"private-session","call_id":"call-4","tool_name":"edit","tool_input":{"path":"C:/private/file","old":"PRIVATE_OLD","new":"PRIVATE_NEW"},"tool_response":{"success":true}}`, "sed -i", "agent-tool edit", "agent-tool", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := NormalizeOpenCodeEvents([]byte(tc.payload), when)
			if len(events) != 1 {
				t.Fatalf("events=%+v", events)
			}
			event := events[0]
			if event.Command != tc.command || event.Signature != tc.signature || event.Tool != tc.tool ||
				event.Harness != "opencode" || event.Success == nil || *event.Success != tc.success {
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

func TestNormalizeOpenCodeEventsIsDeterministicAndFailsOpen(t *testing.T) {
	when := time.Now()
	valid := `{"session_id":"session","tool_name":"bash","tool_input":{"command":"git status"},"tool_response":{"success":true}}`
	first := NormalizeOpenCodeEvents([]byte(valid), when)
	second := NormalizeOpenCodeEvents([]byte(valid), when.Add(time.Minute))
	if len(first) != 1 || len(second) != 1 || first[0].DeliveryKey() != second[0].DeliveryKey() {
		t.Fatalf("fallback identity is not deterministic: %+v %+v", first, second)
	}
	cases := [][]byte{
		nil, []byte(`{`), bytes.Repeat([]byte("x"), MaxHarnessHookBytes+1),
		[]byte(strings.Replace(valid, `"session_id":"session"`, `"session_id":""`, 1)),
		[]byte(strings.Replace(valid, `"tool_name":"bash"`, `"tool_name":"unknown"`, 1)),
		[]byte(strings.Replace(valid, `"session_id":"session"`, `"hook_event_name":"SessionStart","session_id":"session"`, 1)),
	}
	for _, raw := range cases {
		if events := NormalizeOpenCodeEvents(raw, when); len(events) != 0 {
			t.Fatalf("unsafe input produced events: %+v", events)
		}
	}
}
