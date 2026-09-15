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

const OwnerMarker = "--devtrack-sage-hook"

type Installation struct {
	SchemaVersion int    `json:"schema_version"`
	Installed     bool   `json:"installed"`
	Changed       bool   `json:"changed"`
	Mode          string `json:"mode"`
	ConfigPath    string `json:"config_path,omitempty"`
}

type Installer struct {
	Root       string
	CodexHome  string
	Executable string
	GOOS       string
}

func (i Installer) Status() (Installation, error) {
	if i.GOOS == "" {
		i.GOOS = runtime.GOOS
	}
	path, err := i.codexHooksPath()
	if err != nil {
		return Installation{}, err
	}
	if i.GOOS == "windows" {
		installed := sage.CodexHistoryEnabled(i.Root)
		return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: map[bool]string{true: "codex-history", false: "disabled"}[installed], ConfigPath: path}, nil
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
		mode = "codex-hook"
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: installed, Mode: mode, ConfigPath: path}, nil
}

func (i Installer) Install() (Installation, error) {
	if i.GOOS == "" {
		i.GOOS = runtime.GOOS
	}
	if i.GOOS == "windows" {
		before := sage.CodexHistoryEnabled(i.Root)
		changed, path, err := i.removeOwnedCodexHooks()
		if err != nil {
			return Installation{}, err
		}
		if err := sage.SetCodexHistoryEnabled(i.Root, true); err != nil {
			return Installation{}, errors.New("sage history configuration unavailable")
		}
		return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: !before || changed, Mode: "codex-history", ConfigPath: path}, nil
	}
	if strings.TrimSpace(i.Executable) == "" {
		return Installation{}, errors.New("devtrack executable path is required")
	}
	if err := sage.SetCodexHistoryEnabled(i.Root, false); err != nil {
		return Installation{}, errors.New("sage history configuration unavailable")
	}
	path, err := i.codexHooksPath()
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
	removeOwned(document)
	hooksMap := ensureMap(document, "hooks")
	existing, _ := hooksMap["PostToolUse"].([]any)
	command := shellQuote(i.Executable) + " sage hook codex " + OwnerMarker
	entry := map[string]any{
		"matcher": "Bash|Shell",
		"hooks":   []any{map[string]any{"type": "command", "command": command, "timeout": float64(10)}},
	}
	hooksMap["PostToolUse"] = append(existing, entry)
	after, _ := json.Marshal(document)
	changed := missing || !bytes.Equal(before, after)
	if changed {
		if err := writeHookDocument(path, document, mode); err != nil {
			return Installation{}, err
		}
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: true, Changed: changed, Mode: "codex-hook", ConfigPath: path}, nil
}

func (i Installer) Uninstall() (Installation, error) {
	wasEnabled := sage.CodexHistoryEnabled(i.Root)
	if err := sage.SetCodexHistoryEnabled(i.Root, false); err != nil {
		return Installation{}, errors.New("sage history configuration unavailable")
	}
	changed, path, err := i.removeOwnedCodexHooks()
	if err != nil {
		return Installation{}, err
	}
	return Installation{SchemaVersion: sage.SchemaVersion, Installed: false, Changed: wasEnabled || changed, Mode: "disabled", ConfigPath: path}, nil
}

func (i Installer) removeOwnedCodexHooks() (bool, string, error) {
	path, err := i.codexHooksPath()
	if err != nil {
		return false, "", err
	}
	document, mode, err := readHookDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, path, nil
	}
	if err != nil {
		return false, path, err
	}
	changed := removeOwned(document)
	if changed {
		if err := writeHookDocument(path, document, mode); err != nil {
			return false, path, err
		}
	}
	return changed, path, nil
}

func (i Installer) codexHooksPath() (string, error) {
	home := strings.TrimSpace(i.CodexHome)
	if home == "" {
		home = strings.TrimSpace(os.Getenv("CODEX_HOME"))
	}
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("codex home unavailable")
		}
		home = filepath.Join(userHome, ".codex")
	}
	return filepath.Join(home, "hooks.json"), nil
}

func readHookDocument(path string) (map[string]any, os.FileMode, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, 0600, os.ErrNotExist
	}
	if err != nil {
		return nil, 0, errors.New("codex hook configuration unavailable")
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil || document == nil {
		return nil, 0, errors.New("codex hook configuration is invalid")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, errors.New("codex hook configuration unavailable")
	}
	return document, info.Mode().Perm(), nil
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if child, ok := parent[key].(map[string]any); ok {
		return child
	}
	child := map[string]any{}
	parent[key] = child
	return child
}

func removeOwned(document map[string]any) bool {
	hooksMap, ok := document["hooks"].(map[string]any)
	if !ok {
		return false
	}
	changed := false
	for event, value := range hooksMap {
		entries, ok := value.([]any)
		if !ok {
			continue
		}
		kept := entries[:0]
		for _, entry := range entries {
			raw, _ := json.Marshal(entry)
			if strings.Contains(string(raw), OwnerMarker) {
				changed = true
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(hooksMap, event)
		} else {
			hooksMap[event] = kept
		}
	}
	if len(hooksMap) == 0 {
		delete(document, "hooks")
	}
	return changed
}

func containsOwned(document map[string]any) bool {
	raw, _ := json.Marshal(document)
	return strings.Contains(string(raw), OwnerMarker)
}

func writeHookDocument(path string, document map[string]any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("codex hook configuration unavailable")
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hooks-*.tmp")
	if err != nil {
		return errors.New("codex hook configuration unavailable")
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
		return fmt.Errorf("publish codex hook configuration: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
