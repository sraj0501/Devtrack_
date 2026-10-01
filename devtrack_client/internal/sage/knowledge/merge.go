package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mergeFile = ".merge.json"

// A bounded write-ahead record makes a multi-file merge replayable. Recovery
// accepts only the exact before/after snapshots; it never overwrites later edits.
type mergeRecord struct {
	Source, Destination                               string
	SourceBefore, DestinationBefore, DestinationAfter []byte
	RoutesBefore, RoutesAfter                         []byte
	Count                                             int
}

// Merge moves entries with their original prose and signatures. Like Write,
// callers across processes must hold the shared Sage database writer lock.
func (w Writer) Merge(source, destination string) (int, error) {
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return 0, err
	}
	src, dst := SafeFilename(source), SafeFilename(destination)
	previous, err := w.recoverMerge()
	if err != nil {
		return 0, err
	}
	if previous != nil && previous.Source == src && previous.Destination == dst {
		return previous.Count, nil
	}
	if src == dst {
		return 0, nil
	}
	raw, err := w.read(src)
	if err != nil || len(raw) == 0 {
		return 0, err
	}
	dest, err := w.read(dst)
	if err != nil {
		return 0, err
	}
	if !balancedFences(string(raw)) || !balancedFences(string(dest)) {
		return 0, errors.New("unclosed Sage topic code fence")
	}
	entries := ParseMarkdown(string(raw))
	// Retain files with unstructured notes instead of silently deleting prose
	// that is not part of an entry. Users can refile those notes manually.
	if len(entries) == 0 {
		return 0, errors.New("Sage source has no movable entries")
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	covered := make([]bool, len(lines))
	for _, e := range entries {
		for i := e.Start; i < e.End; i++ {
			covered[i] = true
		}
	}
	for i, line := range lines {
		if covered[i] || strings.TrimSpace(line) == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || line == "> Auto-maintained by DevTrack Sage. Signature comments are dedupe keys." {
			continue
		}
		return 0, errors.New("Sage source contains notes outside entries; preserve them before merging")
	}
	saved, err := w.loadRoutes()
	if err != nil {
		return 0, err
	}
	beforeRoutes, err := w.read(routesFile)
	if err != nil {
		return 0, err
	}
	updated := string(dest)
	if len(dest) == 0 {
		updated = fmt.Sprintf("# %s\n", prose(topicTitle(dst, "")))
	}
	for _, entry := range entries {
		body := strings.TrimSpace(strings.Join(lines[entry.Start:entry.End], "\n")) + "\n"
		updated = insertMarkdown(updated, entry.Section, body)
		if binary := entryBinary(entry); binary != "" {
			saved[binary] = dst
		}
	}
	for binary, name := range saved {
		if name == src {
			saved[binary] = dst
		}
	}
	afterRoutes, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return 0, err
	}
	r := mergeRecord{src, dst, raw, dest, []byte(updated), beforeRoutes, append(afterRoutes, '\n'), len(entries)}
	if len(r.DestinationAfter) > maxTopicBytes {
		return 0, errors.New("Sage topic exceeds size limit")
	}
	journal, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}
	if err := w.write(mergeFile, journal); err != nil {
		return 0, err
	}
	_, err = w.recoverMerge()
	return r.Count, err
}

func (w Writer) recoverMerge() (*mergeRecord, error) {
	raw, err := w.read(mergeFile)
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	var r mergeRecord
	if json.Unmarshal(raw, &r) != nil || r.Source == r.Destination ||
		SafeFilename(r.Source) != r.Source || SafeFilename(r.Destination) != r.Destination ||
		len(r.SourceBefore) == 0 || len(r.DestinationAfter) == 0 || r.Count < 1 {
		return nil, errors.New("invalid Sage merge recovery record")
	}
	for _, snapshot := range []struct {
		name          string
		before, after []byte
	}{
		{r.Source, r.SourceBefore, nil}, {r.Destination, r.DestinationBefore, r.DestinationAfter},
		{routesFile, r.RoutesBefore, r.RoutesAfter},
	} {
		current, err := w.read(snapshot.name)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(current, snapshot.before) && !bytes.Equal(current, snapshot.after) {
			return nil, errors.New("Sage merge recovery conflicts with edited files; preserve edits before retrying")
		}
	}
	if err := w.write(r.Destination, r.DestinationAfter); err != nil {
		return nil, err
	}
	if err := w.write(routesFile, r.RoutesAfter); err != nil {
		return nil, err
	}
	// Destination and routes are durable before the source is removed.
	if err := os.Remove(filepath.Join(w.Root, r.Source)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := w.writeIndex(); err != nil {
		return nil, err
	}
	if err := os.Remove(filepath.Join(w.Root, mergeFile)); err != nil {
		return nil, err
	}
	return &r, nil
}
