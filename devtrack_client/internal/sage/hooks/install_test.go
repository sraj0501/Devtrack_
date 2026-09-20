package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestCodexHookInstallIsIdempotentAndPreservesUnrelatedEntries(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	path := filepath.Join(home, "hooks.json")
	original := `{"custom":"keep","hooks":{"PostToolUse":[{"matcher":"Other","hooks":[{"type":"command","command":"other-tool"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"session-tool"}]}]}}`
	if err := os.WriteFile(path, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Root: root, CodexHome: home, Executable: "/opt/dev track/devtrack", GOOS: "linux"}
	first, err := installer.Install()
	if err != nil || !first.Installed || !first.Changed || first.Mode != "codex-hook" {
		t.Fatalf("first: %+v, %v", first, err)
	}
	raw1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw1), OwnerMarker) != 1 || !strings.Contains(string(raw1), "other-tool") || !strings.Contains(string(raw1), "session-tool") || !strings.Contains(string(raw1), `"custom": "keep"`) {
		t.Fatalf("bad merge: %s", raw1)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	second, err := installer.Install()
	if err != nil || second.Changed {
		t.Fatalf("second: %+v, %v", second, err)
	}
	raw2, _ := os.ReadFile(path)
	if string(raw1) != string(raw2) {
		t.Fatal("idempotent install rewrote content")
	}

	removed, err := installer.Uninstall()
	if err != nil || removed.Installed || !removed.Changed {
		t.Fatalf("remove: %+v, %v", removed, err)
	}
	raw3, _ := os.ReadFile(path)
	if strings.Contains(string(raw3), OwnerMarker) || !strings.Contains(string(raw3), "other-tool") || !strings.Contains(string(raw3), "session-tool") {
		t.Fatalf("bad removal: %s", raw3)
	}
	again, err := installer.Uninstall()
	if err != nil || again.Changed {
		t.Fatalf("second remove: %+v, %v", again, err)
	}
}

func TestWindowsInstallUsesHistoryAndRemovesOnlyOwnedHook(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	path := filepath.Join(home, "hooks.json")
	document := map[string]any{"hooks": map[string]any{"PostToolUse": []any{
		map[string]any{"hooks": []any{map[string]any{"command": "devtrack sage hook codex " + OwnerMarker}}},
		map[string]any{"hooks": []any{map[string]any{"command": "warp-user-hook"}}},
	}}}
	raw, _ := json.Marshal(document)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	installer := Installer{Root: root, CodexHome: home, GOOS: "windows"}
	result, err := installer.Install()
	if err != nil || result.Mode != "codex-history" || !sage.CodexHistoryEnabled(root) {
		t.Fatalf("install: %+v, %v", result, err)
	}
	merged, _ := os.ReadFile(path)
	if strings.Contains(string(merged), OwnerMarker) || !strings.Contains(string(merged), "warp-user-hook") {
		t.Fatalf("bad windows merge: %s", merged)
	}
	if _, err := installer.Uninstall(); err != nil || sage.CodexHistoryEnabled(root) {
		t.Fatalf("uninstall: %v", err)
	}
}

func TestInvalidHookConfigFailsClosedWithoutOverwrite(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	path := filepath.Join(home, "hooks.json")
	if err := os.WriteFile(path, []byte(`{invalid`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := (Installer{Root: root, CodexHome: home, Executable: "devtrack", GOOS: "linux"}).Install()
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != `{invalid` {
		t.Fatal("invalid config was overwritten")
	}
}

func TestHookConfigAcceptsUTF8BOM(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	path := filepath.Join(home, "hooks.json")
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"custom":"keep"}`)...)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := (Installer{Root: root, CodexHome: home, GOOS: "windows"}).Install()
	if err != nil || !result.Installed {
		t.Fatalf("install: %+v, %v", result, err)
	}
}
