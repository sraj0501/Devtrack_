package config

import (
	"os"
	"path/filepath"
)

// DevtrackDataHome returns the XDG data home directory for DevTrack.
// Default: ~/.local/share/devtrack. Honours $XDG_DATA_HOME if set.
func DevtrackDataHome() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "devtrack"), nil
}
