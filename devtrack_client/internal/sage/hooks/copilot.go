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

// CopilotInstaller owns only DevTrack's entry in Copilot CLI's dedicated
// version-1 hook file. The DevTrack daemon remains the lifecycle owner.
type CopilotInstaller struct {
	CopilotHome string
	Executable  string
}

func (i CopilotInstaller) Status() (Installation, error) {
	path, err := i.hooksPath()
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
		mode = "copilot-hook"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i CopilotInstaller) Install() (Installation, error) {
	if strings.TrimSpace(i.Executable) == "" {
		return Installation{}, errors.New("devtrack executable path is required")
	}
	path, err := i.hooksPath()
	if err != nil {
		return Installation{}, err
	}
	document, mode, err := readHookDocument(path)
	missing := errors.Is(err, os.ErrNotExist)
	if missing {
		document, mode, err = map[string]any{"version": float64(1)}, 0600, nil
	}
	if err != nil {
		return Installation{}, err
	}
	before, _ := json.Marshal(document)
	removeMarked(document, ownedMarkers(), false)
	hooksMap, err := copilotHooksMap(document)
	if err != nil {
		return Installation{}, err
	}
	commandArgs := " sage hook copilot " + OwnerMarker
	entry := map[string]any{
		"type":       "command",
		"bash":       posixShellQuote(i.Executable) + commandArgs,
		"powershell": powershellInvoke(i.Executable) + commandArgs,
		"env":        map[string]any{"AI_SKILLS_HARNESS": "copilot"},
		"timeoutSec": float64(10),
	}
	existing, _ := hooksMap["postToolUse"].([]any)
	hooksMap["postToolUse"] = append(existing, entry)
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
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "copilot-hook", ConfigPath: path, Outcome: outcome}, nil
}

func (i CopilotInstaller) Uninstall() (Installation, error) {
	path, err := i.hooksPath()
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

func copilotHooksMap(document map[string]any) (map[string]any, error) {
	if version, exists := document["version"]; exists && version != float64(1) {
		return nil, errors.New("copilot hook configuration version is unsupported")
	}
	document["version"] = float64(1)
	value, exists := document["hooks"]
	if !exists {
		value = map[string]any{}
		document["hooks"] = value
	}
	hooksMap, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("copilot hook configuration is invalid")
	}
	if value, exists := hooksMap["postToolUse"]; exists {
		if _, ok := value.([]any); !ok {
			return nil, errors.New("copilot hook configuration is invalid")
		}
	}
	return hooksMap, nil
}

func (i CopilotInstaller) hooksPath() (string, error) {
	home := strings.TrimSpace(i.CopilotHome)
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("copilot home unavailable")
		}
		home = filepath.Join(userHome, ".copilot")
	}
	return filepath.Join(home, "hooks", "ai-session-skills.json"), nil
}

func posixShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func powershellInvoke(value string) string {
	return "& '" + strings.ReplaceAll(value, "'", "''") + "'"
}
