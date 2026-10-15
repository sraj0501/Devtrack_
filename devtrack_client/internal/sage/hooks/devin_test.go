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
)

func TestDevinInstallerAdditiveStableAndSelective(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config.json")
	foreign := `{"theme":"dark","hooks":{"SessionStart":[{"matcher":"","hooks":[{"type":"command","command":"keep-start"}]}],"PostToolUse":[{"matcher":"custom","hooks":[{"type":"command","command":"keep-tool"}]}]}}`
	if err := os.WriteFile(path, []byte(foreign), 0640); err != nil {
		t.Fatal(err)
	}
	installer := DevinInstaller{DevinHome: home, Executable: `C:\Program Files\DevTrack\devtrack.exe`}
	result, err := installer.Install()
	if err != nil || !result.Installed || !result.Changed || result.Mode != "devin-hook" {
		t.Fatalf("install=%+v err=%v", result, err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"keep-start", "keep-tool", devinMatcher, `sage hook devin`, OwnerMarker, `"timeout": 10`} {
		if !strings.Contains(string(first), want) {
			t.Fatalf("missing %q in %s", want, first)
		}
	}
	if strings.Count(string(first), OwnerMarker) != 1 {
		t.Fatalf("owned entries=%d", strings.Count(string(first), OwnerMarker))
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("mode=%v", info.Mode().Perm())
	}
	secondResult, err := installer.Install()
	second, _ := os.ReadFile(path)
	if err != nil || secondResult.Changed || !bytes.Equal(first, second) {
		t.Fatalf("reinstall=%+v err=%v stable=%t", secondResult, err, bytes.Equal(first, second))
	}
	status, err := installer.Status()
	if err != nil || !status.Installed {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	uninstalled, err := installer.Uninstall()
	after, _ := os.ReadFile(path)
	if err != nil || !uninstalled.Changed || strings.Contains(string(after), OwnerMarker) || !strings.Contains(string(after), "keep-tool") || !strings.Contains(string(after), "keep-start") {
		t.Fatalf("uninstall=%+v err=%v config=%s", uninstalled, err, after)
	}
}

func TestDevinInstallerLegacyInvalidAndMissingBehavior(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		home := t.TempDir()
		path := filepath.Join(home, "config.json")
		legacy := `{"hooks":{"PostToolUse":[{"matcher":"exec","hooks":[{"type":"command","command":"python ai_sessions_skills/tool/bin/capture.py"}]}]}}`
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := (DevinInstaller{DevinHome: home, Executable: "devtrack"}).Install(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if strings.Contains(string(raw), "ai_sessions_skills") || strings.Count(string(raw), OwnerMarker) != 1 {
			t.Fatalf("legacy remained: %s", raw)
		}
	})
	for name, body := range map[string]string{"malformed": `{`, "invalid-hooks": `{"hooks":[]}`, "invalid-event": `{"hooks":{"PostToolUse":{}}}`} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "config.json")
			original := []byte(body)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (DevinInstaller{DevinHome: home, Executable: "devtrack"}).Install(); err == nil {
				t.Fatal("invalid config must fail")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(original, after) {
				t.Fatalf("invalid file changed: %q", after)
			}
		})
	}
	t.Run("missing-uninstall", func(t *testing.T) {
		home := t.TempDir()
		result, err := (DevinInstaller{DevinHome: home}).Uninstall()
		if err != nil || result.Outcome != "missing" || result.Changed {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Stat(filepath.Join(home, "config.json")); !os.IsNotExist(err) {
			t.Fatalf("missing uninstall created file: %v", err)
		}
	})
}

func TestDevinInstallerUsesPlatformConfigurationRoot(t *testing.T) {
	home := filepath.Join("home", "person")
	cases := []struct {
		name, goos string
		env        map[string]string
		want       string
	}{
		{"windows-appdata", "windows", map[string]string{"APPDATA": filepath.Join("C:", "Users", "person", "AppData", "Roaming")}, filepath.Join("C:", "Users", "person", "AppData", "Roaming", "devin", "config.json")},
		{"linux-xdg", "linux", map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, ".xdg")}, filepath.Join(home, ".xdg", "devin", "config.json")},
		{"linux-fallback", "linux", map[string]string{}, filepath.Join(home, ".config", "devin", "config.json")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := (DevinInstaller{GOOS: tc.goos, Env: tc.env, UserHome: home}).configPath()
			if err != nil || path != tc.want {
				t.Fatalf("path=%q want=%q err=%v", path, tc.want, err)
			}
		})
	}
}

func TestNormalizeDevinEventsSupportedToolsAndPrivacy(t *testing.T) {
	when := time.Date(2026, 10, 16, 1, 2, 3, 0, time.UTC)
	cases := []string{
		`{"tool_name":"exec","tool_input":{"command":"terraform apply PRIVATE_ARG"},"tool_response":{"success":true,"output":"PRIVATE_OUTPUT"}}`,
		`{"tool_name":"grep","tool_input":{"pattern":"PRIVATE_PATTERN","context_lines":2},"tool_response":{"success":true}}`,
		`{"tool_name":"glob","tool_input":{"pattern":"PRIVATE_GLOB"},"tool_response":{"success":true}}`,
		`{"tool_name":"edit","tool_input":{"file_path":"PRIVATE_PATH","old_string":"PRIVATE_OLD","new_string":"PRIVATE_NEW"},"tool_response":{"success":true}}`,
		`{"tool_name":"write","tool_input":{"file_path":"PRIVATE_PATH","content":"PRIVATE_CONTENT"},"tool_response":{"success":true}}`,
		`{"tool_name":"apply_patch","tool_input":{"patch":"PRIVATE_PATCH"},"tool_response":{"success":true}}`,
	}
	for _, raw := range cases {
		events := NormalizeDevinEvents([]byte(raw), when)
		if len(events) != 1 || events[0].Harness != "devin" || events[0].SessionID == "" || events[0].Success == nil || !*events[0].Success {
			t.Fatalf("events=%+v", events)
		}
		encoded, _ := json.Marshal(events[0])
		if bytes.Contains(bytes.ToUpper(encoded), []byte("PRIVATE")) {
			t.Fatalf("private value leaked: %s", encoded)
		}
	}
}

func TestNormalizeDevinEventsDeterministicAndFailOpen(t *testing.T) {
	when := time.Date(2026, 10, 16, 1, 2, 3, 0, time.UTC)
	payload := []byte(`{"hook_event_name":"PostToolUse","cwd":"C:/private/repo","tool_name":"exec","tool_input":{"command":"git status --short"},"tool_response":{"success":true}}`)
	first := NormalizeDevinEvents(payload, when)
	second := NormalizeDevinEvents(payload, when.Add(time.Hour))
	if len(first) != 1 || len(second) != 1 || first[0].EventID != second[0].EventID || first[0].SessionID != second[0].SessionID {
		t.Fatalf("unstable identities: %+v %+v", first, second)
	}
	bad := [][]byte{nil, []byte(`{`), []byte(`{"hook_event_name":"SessionStart","tool_name":"exec","tool_input":{"command":"git status"}}`), []byte(`{"tool_name":"unknown","tool_input":{"command":"git status"}}`), []byte(`{"tool_name":"exec","tool_input":{}}`), bytes.Repeat([]byte("x"), MaxHarnessHookBytes+1)}
	for _, raw := range bad {
		if events := NormalizeDevinEvents(raw, when); len(events) != 0 {
			t.Fatalf("unexpected events for %q: %+v", raw, events)
		}
	}
}
