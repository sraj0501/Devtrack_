package sage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultImportLimit = 500

type EventSink func(Event) (inserted bool, err error)

type ImportResult struct {
	Imported    int
	Duplicates  int
	Quarantined int
	Remaining   int
}

func pendingDir(root string) string    { return filepath.Join(root, "spool", "pending") }
func quarantineDir(root string) string { return filepath.Join(root, "spool", "quarantine") }

func EnsureSpool(root string) error {
	if err := os.MkdirAll(pendingDir(root), 0700); err != nil {
		return errors.New("sage spool unavailable")
	}
	return nil
}

// WriteEvent durably publishes one immutable, normalized event. Repeated
// deliveries resolve to the same filename and are successful no-ops.
func WriteEvent(root string, event Event) error {
	state, err := ReadState(root)
	if err != nil {
		return errors.New("sage capture state unavailable")
	}
	if state.Paused {
		return nil
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return errors.New("sage event encoding failed")
	}
	if _, err := DecodeEvent(raw); err != nil {
		return err
	}
	dir := pendingDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("sage spool unavailable")
	}
	final := filepath.Join(dir, event.DeliveryKey()+".json")
	if _, err := os.Stat(final); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("sage spool unavailable")
	}
	tmp, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return errors.New("sage spool unavailable")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return errors.New("sage spool unavailable")
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return errors.New("sage spool write failed")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return errors.New("sage spool write failed")
	}
	if err := tmp.Close(); err != nil {
		return errors.New("sage spool write failed")
	}
	// A hard link atomically publishes without replacing an existing delivery.
	if err := os.Link(tmpPath, final); err != nil {
		if _, statErr := os.Stat(final); statErr == nil {
			return nil
		}
		return errors.New("sage spool publish failed")
	}
	return nil
}

// ImportSpool processes at most limit immutable files. Invalid files are
// quarantined; a sink failure leaves its file pending for a later retry.
func ImportSpool(root string, limit int, sink EventSink) (ImportResult, error) {
	var result ImportResult
	if sink == nil {
		return result, errors.New("sage event sink is required")
	}
	state, err := ReadState(root)
	if err != nil || state.Paused {
		return result, err
	}
	if limit <= 0 || limit > DefaultImportLimit {
		limit = DefaultImportLimit
	}
	entries, err := os.ReadDir(pendingDir(root))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, errors.New("sage spool unavailable")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	processed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if processed >= limit {
			result.Remaining++
			continue
		}
		processed++
		path := filepath.Join(pendingDir(root), entry.Name())
		raw, readErr := readBoundedEvent(path)
		var event Event
		if readErr == nil {
			event, readErr = DecodeEvent(raw)
		}
		if readErr != nil {
			if err := quarantine(path, root); err != nil {
				return result, err
			}
			result.Quarantined++
			continue
		}
		inserted, sinkErr := sink(event)
		if sinkErr != nil {
			result.Remaining++
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("sage spool acknowledgement failed")
		}
		if inserted {
			result.Imported++
		} else {
			result.Duplicates++
		}
	}
	return result, nil
}

func readBoundedEvent(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, MaxEventBytes+1))
}

func quarantine(path, root string) error {
	dir := quarantineDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("sage quarantine unavailable")
	}
	destination := filepath.Join(dir, filepath.Base(path))
	if err := os.Rename(path, destination); err != nil {
		return fmt.Errorf("sage quarantine failed: %w", err)
	}
	return nil
}
