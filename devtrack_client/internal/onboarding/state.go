package onboarding

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

const resultFileName = "first-run-profile.json"

// Result records the completed first-run voice-profile build. The file is a
// local, durable marker so daemon restarts never repeat an expensive profile
// generation and status can keep showing the next useful command.
type Result struct {
	CommitCount int       `json:"commit_count"`
	WordCount   int       `json:"word_count"`
	ProfilePath string    `json:"profile_path,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

// ResultPath returns the first-run result marker under the DevTrack data home.
func ResultPath() (string, error) {
	home, err := config.DevtrackDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, resultFileName), nil
}

// ReadResult reads the completed first-run result.
func ReadResult() (*Result, error) {
	path, err := ResultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode first-run profile result: %w", err)
	}
	return &result, nil
}

// WriteResult writes the success marker once. A completed result is preserved
// across later daemon starts; corrupt markers are replaced on the next success.
func WriteResult(result Result) error {
	path, err := ResultPath()
	if err != nil {
		return err
	}
	if _, err := ReadResult(); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	result.CompletedAt = time.Now().UTC()
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".first-run-profile-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
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
	return os.Rename(tmpPath, path)
}
