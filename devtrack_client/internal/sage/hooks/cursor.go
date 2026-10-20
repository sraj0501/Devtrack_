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

const cursorMatcher = "Shell|Write"

// CursorInstaller owns only DevTrack's capture entries in Cursor's user hook
// file. Session lifecycle remains owned by the existing Sage daemon.
type CursorInstaller struct {
	CursorHome string
	Executable string
}

func (i CursorInstaller) Status() (Installation, error) {
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
		mode = "cursor-hook"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i CursorInstaller) Install() (Installation, error) {
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
	hooksMap, err := cursorHooksMap(document)
	if err != nil {
		return Installation{}, err
	}
	command := shellQuote(i.Executable) + " sage hook cursor " + OwnerMarker
	afterShell, _ := hooksMap["afterShellExecution"].([]any)
	hooksMap["afterShellExecution"] = append(afterShell, map[string]any{
		"type": "command", "command": command, "timeout": float64(10),
	})
	postTool, _ := hooksMap["postToolUse"].([]any)
	hooksMap["postToolUse"] = append(postTool, map[string]any{
		"type": "command", "command": command, "matcher": cursorMatcher, "timeout": float64(10),
	})
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
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "cursor-hook", ConfigPath: path, Outcome: outcome}, nil
}

func (i CursorInstaller) Uninstall() (Installation, error) {
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

func cursorHooksMap(document map[string]any) (map[string]any, error) {
	if version, exists := document["version"]; exists && version != float64(1) {
		return nil, errors.New("cursor hook configuration version is unsupported")
	}
	document["version"] = float64(1)
	value, exists := document["hooks"]
	if !exists {
		value = map[string]any{}
		document["hooks"] = value
	}
	hooksMap, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("cursor hook configuration is invalid")
	}
	for _, event := range []string{"afterShellExecution", "postToolUse"} {
		if value, exists := hooksMap[event]; exists {
			if _, ok := value.([]any); !ok {
				return nil, errors.New("cursor hook configuration is invalid")
			}
		}
	}
	return hooksMap, nil
}

func (i CursorInstaller) hooksPath() (string, error) {
	home := strings.TrimSpace(i.CursorHome)
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("cursor home unavailable")
		}
		home = filepath.Join(userHome, ".cursor")
	}
	return filepath.Join(home, "hooks.json"), nil
}
