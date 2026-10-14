package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceWindowsUsesAppdata(t *testing.T) {
	home := filepath.Join("C:", "Users", "me")
	want := filepath.Join(home, "AppData", "Roaming")
	if got := UserConfigDir("windows", map[string]string{"APPDATA": want}, home); got != want {
		t.Fatalf("UserConfigDir() = %q, want %q", got, want)
	}
}

func TestReferenceWindowsFallsBackToHome(t *testing.T) {
	home := filepath.Join("home", "me")
	want := filepath.Join(home, "AppData", "Roaming")
	if got := UserConfigDir("win32", nil, home); got != want {
		t.Fatalf("UserConfigDir() = %q, want %q", got, want)
	}
}

func TestReferenceLinuxHonoursXdg(t *testing.T) {
	want := filepath.Join("home", "me", ".xdg")
	if got := UserConfigDir("linux", map[string]string{"XDG_CONFIG_HOME": want}, filepath.Join("home", "me")); got != want {
		t.Fatalf("UserConfigDir() = %q, want %q", got, want)
	}
}

func TestReferenceLinuxDefaultIsDotConfig(t *testing.T) {
	home := filepath.Join("home", "me")
	if got, want := UserConfigDir("linux", nil, home), filepath.Join(home, ".config"); got != want {
		t.Fatalf("UserConfigDir() = %q, want %q", got, want)
	}
}

func TestReferenceMacosDefaultIsDotConfig(t *testing.T) {
	home := filepath.Join("Users", "me")
	if got, want := UserConfigDir("darwin", nil, home), filepath.Join(home, ".config"); got != want {
		t.Fatalf("UserConfigDir() = %q, want %q", got, want)
	}
}

func TestReferenceLinuxDevinUnderXdg(t *testing.T) {
	home := filepath.Join("home", "me")
	layout := HarnessLayouts("linux", nil, home, nil)
	if got, want := layout["devin"].Hooks, filepath.Join(home, ".config", "devin", "config.json"); got != want {
		t.Fatalf("Devin hooks = %q, want %q", got, want)
	}
	if got, want := layout["opencode"].Plugin, filepath.Join(home, ".config", "opencode", "plugin", "ai-session-skills.js"); got != want {
		t.Fatalf("OpenCode plugin = %q, want %q", got, want)
	}
}

func TestReferenceLinuxRespectsXdgOverride(t *testing.T) {
	home := filepath.Join("home", "me")
	xdg := filepath.Join(home, ".xdg")
	got := HarnessLayouts("linux", map[string]string{"XDG_CONFIG_HOME": xdg}, home, nil)["devin"].Hooks
	if want := filepath.Join(xdg, "devin", "config.json"); got != want {
		t.Fatalf("Devin hooks = %q, want %q", got, want)
	}
}

func TestReferenceWindowsDevinUnderAppdata(t *testing.T) {
	home := filepath.Join("C:", "Users", "me")
	appdata := filepath.Join(home, "AppData", "Roaming")
	got := HarnessLayouts("windows", map[string]string{"APPDATA": appdata}, home, nil)["devin"].Hooks
	if want := filepath.Join(appdata, "devin", "config.json"); got != want {
		t.Fatalf("Devin hooks = %q, want %q", got, want)
	}
}

func TestReferenceDotDirsAreHomeRelativeOnEveryPlatform(t *testing.T) {
	home := filepath.Join("home", "me")
	for _, goos := range []string{"windows", "darwin", "linux"} {
		layout := HarnessLayouts(goos, map[string]string{"APPDATA": filepath.Join("foreign", "appdata")}, home, nil)
		for id, dotdir := range map[string]string{"claude": ".claude", "codex": ".codex", "copilot": ".copilot", "cursor": ".cursor"} {
			if !strings.HasPrefix(layout[id].Hooks, filepath.Join(home, dotdir)+string(filepath.Separator)) {
				t.Fatalf("%s/%s hooks = %q", goos, id, layout[id].Hooks)
			}
		}
	}
}

func TestReferenceWindowsPrefersExistingOpencodeDir(t *testing.T) {
	home := filepath.Join("home", "me")
	want := filepath.Join(home, ".config", "opencode")
	exists := func(path string) bool { return path == want }
	if got := HarnessLayouts("windows", map[string]string{"APPDATA": filepath.Join("appdata")}, home, exists)["opencode"].Dir; got != want {
		t.Fatalf("OpenCode dir = %q, want %q", got, want)
	}
}

func TestReferenceEveryHarnessHasASkillDestination(t *testing.T) {
	for id, layout := range HarnessLayouts("linux", nil, filepath.Join("home", "me"), nil) {
		if !strings.HasSuffix(strings.ToLower(layout.Skill), ".md") {
			t.Fatalf("%s skill = %q", id, layout.Skill)
		}
		if layout.Style != "devin" && layout.Style != "claude" && layout.Style != "plain" {
			t.Fatalf("%s style = %q", id, layout.Style)
		}
	}
}

func TestReferenceLauncherExtensionMatchesPlatform(t *testing.T) {
	if got := NativeLauncherName("windows"); !strings.HasSuffix(got, ".exe") {
		t.Fatalf("Windows launcher = %q", got)
	}
	for _, goos := range []string{"linux", "darwin"} {
		if got := NativeLauncherName(goos); filepath.Ext(got) != "" {
			t.Fatalf("%s launcher = %q", goos, got)
		}
	}
}

func TestReferenceBothPlatformShimsAreShipped(t *testing.T) {
	kinds := NativeLauncherKinds()
	if kinds["windows"] != "native-executable" || kinds["posix"] != "native-executable" {
		t.Fatalf("native launcher families = %#v", kinds)
	}
}
