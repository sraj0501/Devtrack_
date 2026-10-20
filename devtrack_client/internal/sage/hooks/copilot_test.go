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

func TestCopilotInstallIsPortableAdditiveAndByteStable(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hooks", "ai-session-skills.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := `{"version":1,"foreign_empty":{},"hooks":{"postToolUse":[{"type":"command","bash":"foreign-tool"}],"sessionStart":[{"type":"command","bash":"foreign-session"}]}}`
	if err := os.WriteFile(path, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	installer := CopilotInstaller{CopilotHome: home, Executable: `C:\Program Files\Dev'Track\devtrack.exe`}
	first, err := installer.Install()
	if err != nil || !first.Installed || !first.Changed || first.Mode != "copilot-hook" || first.Outcome != "updated" {
		t.Fatalf("install=%+v err=%v", first, err)
	}
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw1)
	for _, expected := range []string{OwnerMarker, `"version": 1`, `"postToolUse"`, `"timeoutSec": 10`,
		`"AI_SKILLS_HARNESS": "copilot"`, `"bash"`, `"powershell"`, "foreign-tool", "foreign-session"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q: %s", expected, raw1)
		}
	}
	var document map[string]any
	if err := json.Unmarshal(raw1, &document); err != nil {
		t.Fatal(err)
	}
	hooksMap := document["hooks"].(map[string]any)
	entries := hooksMap["postToolUse"].([]any)
	owned := entries[len(entries)-1].(map[string]any)
	if strings.Count(text, OwnerMarker) != 2 ||
		owned["bash"] != `'C:\Program Files\Dev'"'"'Track\devtrack.exe' sage hook copilot --devtrack-sage-hook` ||
		owned["powershell"] != `& 'C:\Program Files\Dev''Track\devtrack.exe' sage hook copilot --devtrack-sage-hook` {
		t.Fatalf("portable quoting is invalid: %s", raw1)
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
		t.Fatal("reinstall rewrote Copilot hook file")
	}
	removed, err := installer.Uninstall()
	if err != nil || !removed.Changed || removed.Outcome != "updated" {
		t.Fatalf("uninstall=%+v err=%v", removed, err)
	}
	cleaned, _ := os.ReadFile(path)
	if strings.Contains(string(cleaned), OwnerMarker) || !strings.Contains(string(cleaned), "foreign-tool") ||
		!strings.Contains(string(cleaned), "foreign-session") || !strings.Contains(string(cleaned), `"foreign_empty": {}`) {
		t.Fatalf("uninstall changed foreign data: %s", cleaned)
	}
}

func TestCopilotInstallHandlesMissingInvalidAndLegacyFiles(t *testing.T) {
	t.Run("missing install", func(t *testing.T) {
		home := t.TempDir()
		result, err := (CopilotInstaller{CopilotHome: home, Executable: "devtrack"}).Install()
		if err != nil || !result.Installed || !result.Changed || result.Outcome != "created" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "hooks", "ai-session-skills.json")); err != nil {
			t.Fatalf("install did not create the hook file: %v", err)
		}
	})
	t.Run("missing uninstall", func(t *testing.T) {
		home := t.TempDir()
		result, err := (CopilotInstaller{CopilotHome: home}).Uninstall()
		if err != nil || result.Changed || result.Outcome != "missing" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "hooks", "ai-session-skills.json")); !os.IsNotExist(err) {
			t.Fatalf("uninstall created a file: %v", err)
		}
	})
	t.Run("invalid hooks shape", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "hooks", "ai-session-skills.json")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		original := `{"version":1,"hooks":"foreign"}`
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (CopilotInstaller{CopilotHome: home, Executable: "devtrack"}).Install(); err == nil {
			t.Fatal("invalid hooks shape was accepted")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != original {
			t.Fatal("invalid file was overwritten")
		}
	})
	t.Run("invalid version", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "hooks", "ai-session-skills.json")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		original := `{"version":2,"hooks":{}}`
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (CopilotInstaller{CopilotHome: home, Executable: "devtrack"}).Install(); err == nil {
			t.Fatal("unsupported version was accepted")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != original {
			t.Fatal("invalid file was overwritten")
		}
	})
	t.Run("legacy entry replaced", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "hooks", "ai-session-skills.json")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		legacy := `{"version":1,"hooks":{"postToolUse":[{"bash":"/old/ai_sessions_skills/hook-capture.sh"}]}}`
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (CopilotInstaller{CopilotHome: home, Executable: "devtrack"}).Install(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "ai_sessions_skills") || strings.Count(string(raw), OwnerMarker) != 2 {
			t.Fatalf("legacy entry was not replaced: %s", raw)
		}
	})
}

func TestNormalizeCopilotEventsMinimizesSupportedPayloads(t *testing.T) {
	when := time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC)
	cases := []struct {
		name, payload, command, signature, tool string
		success                                 bool
	}{
		{"shell", `{"hookEventName":"PostToolUse","sessionId":"private-session","cwd":"C:/private/project","timestamp":"2026-10-15T11:00:00Z","toolName":"bash","toolArgs":{"command":"firewall-cmd --list-all"},"toolResult":{"resultType":"success","output":"PRIVATE_OUTPUT"}}`, "firewall-cmd --list-all", "firewall-cmd --list-all", "shell", true},
		{"failed shell", `{"sessionId":"private-session","toolCallId":"call-2","toolName":"bash","toolArgs":"{\"command\":\"go test ./private/package -race\"}","toolResult":{"resultType":"error","error":"PRIVATE_ERROR"}}`, "go test -race", "go test -race", "shell", false},
		{"search", `{"sessionId":"private-session","toolCallId":"call-3","toolName":"grep","toolArgs":{"pattern":"PRIVATE_PATTERN","include":"PRIVATE_GLOB","outputMode":"count"},"toolResult":{"resultType":"success"}}`, "rg --count --glob --regexp", "rg --count --glob --regexp", "agent-tool", true},
		{"edit", `{"sessionId":"private-session","toolCallId":"call-4","toolName":"edit","toolArgs":{"path":"C:/private/file","old":"PRIVATE_OLD","new":"PRIVATE_NEW"},"toolResult":{"resultType":"success"}}`, "sed -i", "agent-tool edit", "agent-tool", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := NormalizeCopilotEvents([]byte(tc.payload), when)
			if len(events) != 1 {
				t.Fatalf("events=%+v", events)
			}
			event := events[0]
			if event.Command != tc.command || event.Signature != tc.signature || event.Tool != tc.tool ||
				event.Harness != "copilot" || event.Success == nil || *event.Success != tc.success {
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

func TestNormalizeCopilotEventsIsDeterministicAndFailsOpen(t *testing.T) {
	when := time.Now()
	valid := `{"sessionId":"session","toolName":"bash","toolArgs":{"command":"git status"},"toolResult":{"resultType":"success"}}`
	first := NormalizeCopilotEvents([]byte(valid), when)
	second := NormalizeCopilotEvents([]byte(valid), when.Add(time.Minute))
	if len(first) != 1 || len(second) != 1 || first[0].DeliveryKey() != second[0].DeliveryKey() {
		t.Fatalf("fallback identity is not deterministic: %+v %+v", first, second)
	}
	cases := [][]byte{
		nil, []byte(`{`), bytes.Repeat([]byte("x"), MaxHarnessHookBytes+1),
		[]byte(strings.Replace(valid, `"sessionId":"session"`, `"sessionId":""`, 1)),
		[]byte(strings.Replace(valid, `"toolName":"bash"`, `"toolName":"unknown"`, 1)),
		[]byte(strings.Replace(valid, `"sessionId":"session"`, `"hookEventName":"SessionStart","sessionId":"session"`, 1)),
	}
	for _, raw := range cases {
		if events := NormalizeCopilotEvents(raw, when); len(events) != 0 {
			t.Fatalf("unsafe input produced events: %+v", events)
		}
	}
}
