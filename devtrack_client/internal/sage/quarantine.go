package sage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const MaxQuarantineEntries = 500

var quarantineIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

type QuarantineEntry struct {
	ID       string    `json:"id"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Reason   string    `json:"reason"`
}

type QuarantineList struct {
	Entries   []QuarantineEntry `json:"entries"`
	Total     int               `json:"total"`
	Truncated bool              `json:"truncated"`
}

type QuarantinePurge struct {
	Removed int `json:"removed"`
}

func quarantineOpaqueID(name string) string {
	digest := sha256.Sum256([]byte(name))
	return hex.EncodeToString(digest[:8])
}

// ListQuarantine returns bounded metadata and fixed diagnostic categories. It
// never returns filenames, file contents, decoder errors, or captured values.
func ListQuarantine(root string) (QuarantineList, error) {
	var result QuarantineList
	dir := quarantineDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		result.Entries = []QuarantineEntry{}
		return result, nil
	}
	if err != nil {
		return result, errors.New("sage quarantine unavailable")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		result.Total++
		if len(result.Entries) >= MaxQuarantineEntries {
			result.Truncated = true
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, statErr := os.Lstat(path)
		item := QuarantineEntry{ID: quarantineOpaqueID(entry.Name()), Reason: "unreadable"}
		if statErr == nil {
			item.Size = info.Size()
			item.Modified = info.ModTime().UTC()
			switch {
			case info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular():
				item.Reason = "unsafe_file"
			case info.Size() > MaxEventBytes:
				item.Reason = "oversized_event"
			default:
				raw, readErr := readBoundedEvent(path)
				if readErr != nil {
					item.Reason = "unreadable"
				} else if _, decodeErr := DecodeEvent(raw); decodeErr != nil {
					item.Reason = "invalid_event"
				} else {
					item.Reason = "quarantined_event"
				}
			}
		}
		result.Entries = append(result.Entries, item)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].ID < result.Entries[j].ID })
	return result, nil
}

// PurgeQuarantine removes either one opaque entry ID or every quarantined JSON
// file. Removing a symlink removes the link itself and never follows it.
func PurgeQuarantine(root, id string, all bool) (QuarantinePurge, error) {
	var result QuarantinePurge
	if all == (id != "") {
		return result, errors.New("choose one quarantine ID or --all")
	}
	if id != "" && !quarantineIDPattern.MatchString(id) {
		return result, errors.New("invalid quarantine ID")
	}
	dir := quarantineDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		if id != "" {
			return result, errors.New("quarantine entry not found")
		}
		return result, nil
	}
	if err != nil {
		return result, errors.New("sage quarantine unavailable")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		if !all && quarantineOpaqueID(entry.Name()) != id {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return result, errors.New("sage quarantine unavailable")
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, errors.New("sage quarantine purge failed")
		}
		result.Removed++
		if !all {
			return result, nil
		}
	}
	if id != "" {
		return result, errors.New("quarantine entry not found")
	}
	return result, nil
}
