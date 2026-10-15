package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

const devinMatcher = "^(exec|grep|glob|edit|write|apply_patch)$"

// DevinInstaller owns only DevTrack's PostToolUse capture entry. The existing
// Sage daemon owns session lifecycle, so no SessionStart or SessionEnd hook is
// installed.
type DevinInstaller struct {
	DevinHome  string
	Executable string
	GOOS       string
	Env        map[string]string
	UserHome   string
}

func (i DevinInstaller) Status() (Installation, error) {
	path, err := i.configPath()
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
		mode = "devin-hook"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i DevinInstaller) Install() (Installation, error) {
	if strings.TrimSpace(i.Executable) == "" {
		return Installation{}, errors.New("devtrack executable path is required")
	}
	path, err := i.configPath()
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
	hooksMap, err := devinHooksMap(document)
	if err != nil {
		return Installation{}, err
	}
	command := shellQuote(i.Executable) + " sage hook devin " + OwnerMarker
	existing, _ := hooksMap["PostToolUse"].([]any)
	hooksMap["PostToolUse"] = append(existing, map[string]any{
		"matcher": devinMatcher,
		"hooks": []any{map[string]any{
			"type": "command", "command": command, "timeout": float64(10),
		}},
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
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "devin-hook", ConfigPath: path, Outcome: outcome}, nil
}

func (i DevinInstaller) Uninstall() (Installation, error) {
	path, err := i.configPath()
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

func devinHooksMap(document map[string]any) (map[string]any, error) {
	value, exists := document["hooks"]
	if !exists {
		value = map[string]any{}
		document["hooks"] = value
	}
	hooksMap, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("devin hook configuration is invalid")
	}
	if value, exists := hooksMap["PostToolUse"]; exists {
		if _, ok := value.([]any); !ok {
			return nil, errors.New("devin hook configuration is invalid")
		}
	}
	return hooksMap, nil
}

func (i DevinInstaller) configPath() (string, error) {
	if home := strings.TrimSpace(i.DevinHome); home != "" {
		return filepath.Join(home, "config.json"), nil
	}
	home := strings.TrimSpace(i.UserHome)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", errors.New("devin home unavailable")
		}
	}
	goos := i.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	env := i.Env
	if env == nil {
		env = map[string]string{"APPDATA": os.Getenv("APPDATA"), "XDG_CONFIG_HOME": os.Getenv("XDG_CONFIG_HOME")}
	}
	return filepath.Join(UserConfigDir(goos, env, home), "devin", "config.json"), nil
}
