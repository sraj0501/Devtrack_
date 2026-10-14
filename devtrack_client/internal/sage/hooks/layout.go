package hooks

import (
	"path/filepath"
	"strings"
)

// HarnessLayout describes the user-owned configuration and skill locations
// for one supported harness. It is deliberately pure so every target layout
// can be verified from Windows or Linux CI without touching a real home.
type HarnessLayout struct {
	Dir    string
	Hooks  string
	Skill  string
	Style  string
	Plugin string
	TOML   string
}

// UserConfigDir returns the platform convention used by harnesses that do not
// keep a home-relative dot directory.
func UserConfigDir(goos string, env map[string]string, home string) string {
	if isWindows(goos) {
		if value := strings.TrimSpace(env["APPDATA"]); value != "" {
			return filepath.Clean(value)
		}
		return filepath.Join(home, "AppData", "Roaming")
	}
	if value := strings.TrimSpace(env["XDG_CONFIG_HOME"]); value != "" {
		return filepath.Clean(value)
	}
	return filepath.Join(home, ".config")
}

// HarnessLayouts returns reference-compatible paths without advertising the
// corresponding adapter as installed. The selective registry remains the
// authority for which adapters are currently available to users.
func HarnessLayouts(goos string, env map[string]string, home string, exists func(string) bool) map[string]HarnessLayout {
	if env == nil {
		env = map[string]string{}
	}
	if exists == nil {
		exists = func(string) bool { return false }
	}
	configRoot := UserConfigDir(goos, env, home)
	devin := filepath.Join(configRoot, "devin")
	opencode := filepath.Join(configRoot, "opencode")
	if isWindows(goos) {
		homeOpenCode := filepath.Join(home, ".config", "opencode")
		if !exists(opencode) && exists(homeOpenCode) {
			opencode = homeOpenCode
		}
	}
	return map[string]HarnessLayout{
		"devin": {
			Dir: devin, Hooks: filepath.Join(devin, "config.json"),
			Skill: filepath.Join(devin, "skills", "session-skills", "SKILL.md"), Style: "devin",
		},
		"claude": {
			Dir: filepath.Join(home, ".claude"), Hooks: filepath.Join(home, ".claude", "settings.json"),
			Skill: filepath.Join(home, ".claude", "skills", "session-skills", "SKILL.md"), Style: "claude",
		},
		"codex": {
			Dir: filepath.Join(home, ".codex"), Hooks: filepath.Join(home, ".codex", "hooks.json"),
			Skill: filepath.Join(home, ".codex", "skills", "session-skills", "SKILL.md"), Style: "plain",
			TOML: filepath.Join(home, ".codex", "config.toml"),
		},
		"copilot": {
			Dir: filepath.Join(home, ".copilot"), Hooks: filepath.Join(home, ".copilot", "hooks", "ai-session-skills.json"),
			Skill: filepath.Join(home, ".copilot", "skills", "session-skills", "SKILL.md"), Style: "plain",
		},
		"cursor": {
			Dir: filepath.Join(home, ".cursor"), Hooks: filepath.Join(home, ".cursor", "hooks.json"),
			Skill: filepath.Join(home, ".cursor", "skills", "session-skills", "SKILL.md"), Style: "plain",
		},
		"opencode": {
			Dir: opencode, Plugin: filepath.Join(opencode, "plugin", "ai-session-skills.js"),
			Skill: filepath.Join(opencode, "command", "session-skills.md"), Style: "plain",
		},
	}
}

// NativeLauncherName is the packaged Go launcher for hook commands. This is
// the intentional replacement for the reference repository's Python .cmd and
// .sh shims.
func NativeLauncherName(goos string) string {
	if isWindows(goos) {
		return "devtrack.exe"
	}
	return "devtrack"
}

// NativeLauncherKinds describes both packaged execution families without
// adding script shims or a Python runtime dependency.
func NativeLauncherKinds() map[string]string {
	return map[string]string{"windows": "native-executable", "posix": "native-executable"}
}

func isWindows(goos string) bool {
	return strings.EqualFold(goos, "windows") || strings.EqualFold(goos, "win32")
}
