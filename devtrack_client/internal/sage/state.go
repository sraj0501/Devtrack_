package sage

import (
	"errors"
	"os"
	"path/filepath"
)

// State is the stable machine-readable SAGE-001 status/doctor response.
// Capture is intentionally unavailable until a harness adapter ships.
type State struct {
	SchemaVersion int    `json:"schema_version"`
	Capture       string `json:"capture"`
	Paused        bool   `json:"paused"`
	Ready         bool   `json:"ready"`
}

const pauseMarker = "paused"

func ReadState(root string) (State, error) {
	state := State{SchemaVersion: SchemaVersion, Capture: "not_installed"}
	_, err := os.Stat(filepath.Join(root, pauseMarker))
	if err == nil {
		state.Paused = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	return state, nil
}

// SetPaused is idempotent. The marker is local and never synchronised.
func SetPaused(root string, paused bool) (State, error) {
	path := filepath.Join(root, pauseMarker)
	if paused {
		if err := os.MkdirAll(root, 0700); err != nil {
			return State{}, err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return State{}, err
		}
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				return State{}, closeErr
			}
		}
	} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return State{}, err
	}
	return ReadState(root)
}
