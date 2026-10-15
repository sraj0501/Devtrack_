package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

// OpenCodeInstaller owns one dedicated OpenCode plugin file. DevTrack's
// existing daemon remains the lifecycle owner; the plugin only forwards tool
// payloads to the bounded native hook endpoint.
type OpenCodeInstaller struct {
	OpenCodeHome string
	Executable   string
	GOOS         string
	Env          map[string]string
	Home         string
	Exists       func(string) bool
}

func (i OpenCodeInstaller) Status() (Installation, error) {
	path, err := i.pluginPath()
	if err != nil {
		return Installation{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Installation{SchemaVersion: sage.SchemaVersion, Mode: "disabled", ConfigPath: path}, nil
	}
	if err != nil {
		return Installation{}, errors.New("opencode plugin unavailable")
	}
	installed := bytes.Contains(raw, []byte(OwnerMarker))
	mode := "disabled"
	if installed {
		mode = "opencode-plugin"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i OpenCodeInstaller) Install() (Installation, error) {
	if strings.TrimSpace(i.Executable) == "" {
		return Installation{}, errors.New("devtrack executable path is required")
	}
	path, err := i.pluginPath()
	if err != nil {
		return Installation{}, err
	}
	want, err := openCodePlugin(i.Executable)
	if err != nil {
		return Installation{}, err
	}
	before, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	mode := os.FileMode(0600)
	if !missing && err != nil {
		return Installation{}, errors.New("opencode plugin unavailable")
	}
	if !missing {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return Installation{}, errors.New("opencode plugin unavailable")
		}
		mode = info.Mode().Perm()
		if !bytes.Contains(before, []byte(OwnerMarker)) && !containsMarker(string(before), ownedMarkers()[1:]) {
			return Installation{}, errors.New("opencode plugin path contains an unowned file")
		}
	}
	changed := missing || !bytes.Equal(before, want)
	if changed {
		if err := writePluginFile(path, want, mode); err != nil {
			return Installation{}, err
		}
	}
	outcome := "unchanged"
	if changed {
		outcome = "updated"
		if missing {
			outcome = "created"
		}
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "opencode-plugin", ConfigPath: path, Outcome: outcome}, nil
}

func (i OpenCodeInstaller) Uninstall() (Installation, error) {
	path, err := i.pluginPath()
	if err != nil {
		return Installation{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Installation{SchemaVersion: sage.SchemaVersion, Mode: "disabled", ConfigPath: path, Outcome: "missing"}, nil
	}
	if err != nil {
		return Installation{}, errors.New("opencode plugin unavailable")
	}
	owned := bytes.Contains(raw, []byte(OwnerMarker)) || containsMarker(string(raw), ownedMarkers()[1:])
	if !owned {
		return Installation{SchemaVersion: sage.SchemaVersion, Mode: "disabled", ConfigPath: path, Outcome: "unchanged"}, nil
	}
	if err := os.Remove(path); err != nil {
		return Installation{}, errors.New("opencode plugin unavailable")
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Changed: true, Mode: "disabled", ConfigPath: path, Outcome: "updated"}, nil
}

func (i OpenCodeInstaller) pluginPath() (string, error) {
	if home := strings.TrimSpace(i.OpenCodeHome); home != "" {
		return filepath.Join(home, "plugin", "ai-session-skills.js"), nil
	}
	goos := i.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	home := strings.TrimSpace(i.Home)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", errors.New("opencode home unavailable")
		}
	}
	env := i.Env
	if env == nil {
		env = map[string]string{"APPDATA": os.Getenv("APPDATA"), "XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME")}
	}
	exists := i.Exists
	if exists == nil {
		exists = func(path string) bool { _, err := os.Stat(path); return err == nil }
	}
	return HarnessLayouts(goos, env, home, exists)["opencode"].Plugin, nil
}

func openCodePlugin(executable string) ([]byte, error) {
	exe, err := json.Marshal(executable)
	if err != nil {
		return nil, err
	}
	source := fmt.Sprintf(`/** DevTrack Sage OpenCode capture plugin. %s */
const DEVTRACK = %s;
const ARGS = ["sage", "hook", "opencode", "%s"];

function spawnCapture(stdin) {
  try {
    if (typeof Bun !== "undefined" && Bun.spawn) {
      const child = Bun.spawn([DEVTRACK, ...ARGS], { stdin: "pipe", stdout: "ignore", stderr: "ignore" });
      child.stdin.write(stdin);
      child.stdin.end();
      child.unref?.();
      return;
    }
  } catch {}
  try {
    import("node:child_process").then(({ spawn }) => {
      try {
        const child = spawn(DEVTRACK, ARGS, { stdio: ["pipe", "ignore", "ignore"], detached: true, windowsHide: true });
        child.stdin.end(stdin);
        child.unref();
      } catch {}
    }).catch(() => {});
  } catch {}
}

function capture(tool, args, sessionID, ok) {
  if (!tool || !args || !sessionID) return;
  try {
    spawnCapture(JSON.stringify({
      hook_event_name: "PostToolUse",
      tool_name: tool,
      tool_input: args,
      session_id: sessionID,
      cwd: process.env.OPENCODE_PROJECT_DIR || process.cwd(),
      tool_response: { success: ok !== false },
    }));
  } catch {}
}

export const DevTrackSage = async () => ({
  "tool.execute.before": async (input, output) => {
    capture(input?.tool, output?.args ?? input?.args, input?.sessionID, true);
  },
  "tool.execute.after": async (input, output) => {
    const failed = typeof output?.output === "string" && /^(error|Error:)/.test(output.output.trim());
    capture(input?.tool, input?.args, input?.sessionID, !failed);
  },
});

export default DevTrackSage;
`, OwnerMarker, exe, OwnerMarker)
	return []byte(source), nil
}

func writePluginFile(path string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("opencode plugin unavailable")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".plugin-*.tmp")
	if err != nil {
		return errors.New("opencode plugin unavailable")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish opencode plugin: %w", err)
	}
	return nil
}
