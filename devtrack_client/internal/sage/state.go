package sage

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// State is the stable machine-readable SAGE-001 status/doctor response.
// Capture is intentionally unavailable until a harness adapter ships.
type State struct {
	SchemaVersion int    `json:"schema_version"`
	Capture       string `json:"capture"`
	Paused        bool   `json:"paused"`
	PauseReason   string `json:"pause_reason,omitempty"`
	Ready         bool   `json:"ready"`
	Backlog       int    `json:"backlog"`
	Quarantined   int    `json:"quarantined"`
	CodexHistory  bool   `json:"codex_history"`
}

const pauseMarker = "paused"
const captureCutoffMarker = "capture-cutoff-ms"
const codexHistoryMarker = "codex-history-enabled"

func ReadState(root string) (State, error) {
	state := State{SchemaVersion: SchemaVersion, Capture: "not_installed"}
	state.CodexHistory = CodexHistoryEnabled(root)
	pausePath := filepath.Join(root, pauseMarker)
	pauseNote, err := os.ReadFile(pausePath)
	if err == nil {
		state.Paused = true
		state.PauseReason = strings.TrimSpace(string(pauseNote))
	} else if !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	if entries, err := os.ReadDir(pendingDir(root)); err == nil {
		state.Capture = "spool"
		state.Ready = !state.Paused
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				state.Backlog++
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	if entries, err := os.ReadDir(quarantineDir(root)); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				state.Quarantined++
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	return state, nil
}

// SetPaused is idempotent. The marker is local and never synchronised.
func SetPaused(root string, paused bool) (State, error) {
	return SetPausedWithReason(root, paused, "")
}

// SetPausedWithReason persists an optional local explanation for a pause.
// The marker remains private runtime state and is never synchronised.
func SetPausedWithReason(root string, paused bool, reason string) (State, error) {
	path := filepath.Join(root, pauseMarker)
	if paused {
		if err := os.MkdirAll(root, 0700); err != nil {
			return State{}, err
		}
		if err := os.WriteFile(path, []byte(strings.TrimSpace(reason)), 0600); err != nil {
			return State{}, err
		}
	} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	// Advancing the cutoff on both transitions prevents a history adapter from
	// backfilling commands that occurred while capture was paused.
	if err := os.MkdirAll(root, 0700); err != nil {
		return State{}, err
	}
	if err := os.WriteFile(filepath.Join(root, captureCutoffMarker), []byte(strconv.FormatInt(time.Now().UnixMilli(), 10)), 0600); err != nil {
		return State{}, err
	}
	return ReadState(root)
}

func CaptureCutoff(root string) (int64, error) {
	raw, err := os.ReadFile(filepath.Join(root, captureCutoffMarker))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("sage capture cutoff is invalid")
	}
	return value, nil
}

func CodexHistoryEnabled(root string) bool {
	_, err := os.Stat(filepath.Join(root, codexHistoryMarker))
	return err == nil
}

func SetCodexHistoryEnabled(root string, enabled bool) error {
	path := filepath.Join(root, codexHistoryMarker)
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}
