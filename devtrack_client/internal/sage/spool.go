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

type CaptureResult struct {
	New    int
	Known  int
	Repeat int
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
	_, err := WriteEvents(root, []Event{event})
	return err
}

// WriteEvents publishes one hook payload as a batch. It records a bounded,
// privacy-safe decision for every valid event, suppressing repeated actions
// within the batch while retaining later, distinct uses for occurrence counts.
func WriteEvents(root string, events []Event) (CaptureResult, error) {
	var result CaptureResult
	if len(events) == 0 {
		return result, nil
	}
	state, err := ReadState(root)
	if err != nil {
		return result, errors.New("sage capture state unavailable")
	}
	if state.Paused {
		return result, nil
	}
	dir := pendingDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return result, errors.New("sage spool unavailable")
	}
	batch := make(map[string]bool, len(events))
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			return result, errors.New("sage event encoding failed")
		}
		if _, err := DecodeEvent(raw); err != nil {
			return result, err
		}
		final := filepath.Join(dir, event.DeliveryKey()+".json")
		if _, err := os.Stat(final); err == nil {
			result.Known++
			recordCaptureDecision(root, "known", event)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("sage spool unavailable")
		}
		if batch[event.Signature] {
			result.Repeat++
			recordCaptureDecision(root, "repeat", event)
			continue
		}
		published, err := publishEvent(dir, final, raw)
		if err != nil {
			return result, err
		}
		if !published {
			result.Known++
			recordCaptureDecision(root, "known", event)
			continue
		}
		batch[event.Signature] = true
		result.New++
		recordCaptureDecision(root, "new", event)
	}
	return result, nil
}

func publishEvent(dir, final string, raw []byte) (bool, error) {
	tmp, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return false, errors.New("sage spool unavailable")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return false, errors.New("sage spool unavailable")
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return false, errors.New("sage spool write failed")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, errors.New("sage spool write failed")
	}
	if err := tmp.Close(); err != nil {
		return false, errors.New("sage spool write failed")
	}
	// A hard link atomically publishes without replacing an existing delivery.
	if err := os.Link(tmpPath, final); err != nil {
		if _, statErr := os.Stat(final); statErr == nil {
			return false, nil
		}
		return false, errors.New("sage spool publish failed")
	}
	return true, nil
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
