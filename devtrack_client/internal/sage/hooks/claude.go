package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

const claudeMatcher = "Bash|Grep|Glob|Edit|MultiEdit|Write|NotebookEdit"

// ClaudeInstaller owns only the DevTrack command hooks in Claude's user
// settings. Session lifecycle remains owned by the existing Sage daemon.
type ClaudeInstaller struct {
	ClaudeHome string
	Executable string
}

func (i ClaudeInstaller) Status() (Installation, error) {
	path, err := i.settingsPath()
	if err != nil {
		return Installation{}, err
	}
	document, _, err := readHookDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return Installation{SchemaVersion: sage.SchemaVersion, Mode: "disabled", ConfigPath: path}, nil
	}
	if err != nil {
		return Installation{}, err
	}
	installed := containsOwned(document)
	mode := "disabled"
	if installed {
		mode = "claude-hook"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i ClaudeInstaller) Install() (Installation, error) {
	if strings.TrimSpace(i.Executable) == "" {
		return Installation{}, errors.New("devtrack executable path is required")
	}
	path, err := i.settingsPath()
	if err != nil {
		return Installation{}, err
	}
	document, mode, err := readHookDocument(path)
	missing := errors.Is(err, os.ErrNotExist)
	if missing {
		document, mode, err = map[string]any{}, 0600, nil
	}
	if err != nil {
		return Installation{}, err
	}
	before, _ := json.Marshal(document)
	removeMarked(document, ownedMarkers(), false)
	hooksMap, err := claudeHooksMap(document)
	if err != nil {
		return Installation{}, err
	}
	command := shellQuote(i.Executable) + " sage hook claude " + OwnerMarker
	appendEntry := func(event, matcher string) {
		existing, _ := hooksMap[event].([]any)
		entry := map[string]any{
			"matcher": matcher,
			"hooks": []any{map[string]any{
				"type": "command", "command": command, "timeout": float64(10),
			}},
		}
		hooksMap[event] = append(existing, entry)
	}
	appendEntry("PostToolUse", claudeMatcher)
	appendEntry("PostToolUseFailure", "Bash")
	after, _ := json.Marshal(document)
	changed := missing || !bytes.Equal(before, after)
	if changed {
		if err := writeHookDocument(path, document, mode); err != nil {
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
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "claude-hook", ConfigPath: path, Outcome: outcome}, nil
}

func (i ClaudeInstaller) Uninstall() (Installation, error) {
	path, err := i.settingsPath()
	if err != nil {
		return Installation{}, err
	}
	document, mode, err := readHookDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return Installation{SchemaVersion: sage.SchemaVersion, Mode: "disabled", ConfigPath: path, Outcome: "missing"}, nil
	}
	if err != nil {
		return Installation{}, err
	}
	changed := removeMarked(document, ownedMarkers(), false)
	if changed {
		if err := writeHookDocument(path, document, mode); err != nil {
			return Installation{}, err
		}
	}
	outcome := "unchanged"
	if changed {
		outcome = "updated"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Changed: changed, Mode: "disabled", ConfigPath: path, Outcome: outcome}, nil
}

func claudeHooksMap(document map[string]any) (map[string]any, error) {
	value, exists := document["hooks"]
	if !exists {
		value = map[string]any{}
		document["hooks"] = value
	}
	hooksMap, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("claude hook configuration is invalid")
	}
	for _, event := range []string{"PostToolUse", "PostToolUseFailure"} {
		if value, exists := hooksMap[event]; exists {
			if _, ok := value.([]any); !ok {
				return nil, errors.New("claude hook configuration is invalid")
			}
		}
	}
	return hooksMap, nil
}

func (i ClaudeInstaller) settingsPath() (string, error) {
	home := strings.TrimSpace(i.ClaudeHome)
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("claude home unavailable")
		}
		home = filepath.Join(userHome, ".claude")
	}
	return filepath.Join(home, "settings.json"), nil
}
