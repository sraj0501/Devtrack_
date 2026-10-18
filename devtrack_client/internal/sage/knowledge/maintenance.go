package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxSnapshotBytes = 64 << 20

type RepairResult struct {
	Topics  int `json:"topics"`
	Entries int `json:"entries"`
}

type ExportResult struct {
	Files   int `json:"files"`
	Entries int `json:"entries"`
}

type ImportResult struct {
	Added     int `json:"added"`
	Unchanged int `json:"unchanged"`
}

// Repair validates the complete Markdown set, recovers an interrupted merge,
// and regenerates only Sage's owned topic-index block.
func (w Writer) Repair() (RepairResult, error) {
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return RepairResult{}, err
	}
	if _, err := w.recoverMerge(); err != nil {
		return RepairResult{}, err
	}
	files, err := readMarkdownDirectory(w.Root)
	if err != nil {
		return RepairResult{}, err
	}
	result := summarizeMarkdown(files)
	if err := w.writeIndex(); err != nil {
		return RepairResult{}, err
	}
	return result, nil
}

// DeleteTopic deletes exactly one portable topic, removes remembered routes to
// it, and repairs the generated index. It never accepts a path or fuzzy name.
func (w Writer) DeleteTopic(topic string) (int, error) {
	name, err := exactTopicFilename(topic)
	if err != nil {
		return 0, err
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return 0, err
	}
	if _, err := w.recoverMerge(); err != nil {
		return 0, err
	}
	raw, err := w.read(name)
	if err != nil {
		return 0, err
	}
	if raw == nil {
		return 0, errors.New("Sage topic not found")
	}
	if !balancedFences(string(raw)) {
		return 0, errors.New("unclosed Sage topic code fence")
	}
	count := len(ParseMarkdown(string(raw)))
	routes, err := w.loadRoutes()
	if err != nil {
		return 0, err
	}
	originalRoutes, err := w.read(routesFile)
	if err != nil {
		return 0, err
	}
	originalIndex, err := w.read("README.md")
	if err != nil {
		return 0, err
	}
	rollback := func(cause error) error {
		var rollbackErrors []string
		if err := atomicReplace(filepath.Join(w.Root, name), raw); err != nil {
			rollbackErrors = append(rollbackErrors, "topic")
		}
		if err := restoreMaintenanceFile(filepath.Join(w.Root, routesFile), originalRoutes); err != nil {
			rollbackErrors = append(rollbackErrors, "routes")
		}
		if err := restoreMaintenanceFile(filepath.Join(w.Root, "README.md"), originalIndex); err != nil {
			rollbackErrors = append(rollbackErrors, "index")
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf("%w (rollback failed for %s)", cause, strings.Join(rollbackErrors, ", "))
		}
		return cause
	}
	if err := w.remove(name); err != nil {
		return 0, err
	}
	changed := false
	for binary, target := range routes {
		if target == name {
			delete(routes, binary)
			changed = true
		}
	}
	if changed {
		raw, err := json.MarshalIndent(routes, "", "  ")
		if err != nil {
			return 0, rollback(err)
		}
		if err := w.write(routesFile, append(raw, '\n')); err != nil {
			return 0, rollback(err)
		}
	}
	if err := w.writeIndex(); err != nil {
		return 0, rollback(err)
	}
	return count, nil
}

// ExportMarkdown writes a new deterministic Markdown-only directory. Existing
// destinations and overlapping source/destination roots are rejected.
func (w Writer) ExportMarkdown(destination string) (ExportResult, error) {
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return ExportResult{}, err
	}
	if _, err := w.recoverMerge(); err != nil {
		return ExportResult{}, err
	}
	files, err := readMarkdownDirectory(w.Root)
	if err != nil {
		return ExportResult{}, err
	}
	sourceAbs, err := filepath.Abs(w.Root)
	if err != nil {
		return ExportResult{}, err
	}
	destinationAbs, err := filepath.Abs(strings.TrimSpace(destination))
	if err != nil || strings.TrimSpace(destination) == "" {
		return ExportResult{}, errors.New("Sage export directory is required")
	}
	if pathsOverlap(sourceAbs, destinationAbs) {
		return ExportResult{}, errors.New("Sage export directory overlaps knowledge directory")
	}
	if _, err := os.Lstat(destinationAbs); err == nil {
		return ExportResult{}, errors.New("Sage export directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return ExportResult{}, errors.New("Sage export directory unavailable")
	}
	parent := filepath.Dir(destinationAbs)
	if err := validateDirectoryChain(parent); err != nil {
		return ExportResult{}, err
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return ExportResult{}, errors.New("Sage export directory unavailable")
	}
	temporary, err := os.MkdirTemp(parent, ".sage-export-*")
	if err != nil {
		return ExportResult{}, errors.New("Sage export directory unavailable")
	}
	defer os.RemoveAll(temporary)
	for _, name := range sortedMarkdownNames(files) {
		if err := os.WriteFile(filepath.Join(temporary, name), files[name], 0600); err != nil {
			return ExportResult{}, errors.New("Sage export write failed")
		}
	}
	if err := os.Rename(temporary, destinationAbs); err != nil {
		return ExportResult{}, errors.New("Sage export publish failed")
	}
	result := ExportResult{Files: len(files)}
	for name, raw := range files {
		if name != "README.md" {
			result.Entries += len(ParseMarkdown(string(raw)))
		}
	}
	return result, nil
}

// ImportMarkdown validates the complete source before writing. It adds only
// missing Markdown files; byte-identical repeats are no-ops and any conflict
// rejects the entire preflight without changing destination files.
func (w Writer) ImportMarkdown(source string) (ImportResult, error) {
	sourceAbs, err := filepath.Abs(strings.TrimSpace(source))
	if err != nil || strings.TrimSpace(source) == "" {
		return ImportResult{}, errors.New("Sage import directory is required")
	}
	targetAbs, err := filepath.Abs(w.Root)
	if err != nil {
		return ImportResult{}, err
	}
	if pathsOverlap(sourceAbs, targetAbs) {
		return ImportResult{}, errors.New("Sage import directory overlaps knowledge directory")
	}
	files, err := readMarkdownDirectory(sourceAbs)
	if err != nil {
		return ImportResult{}, err
	}
	delete(files, "README.md")

	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return ImportResult{}, err
	}
	if _, err := w.recoverMerge(); err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{}
	originalIndex, err := w.read("README.md")
	if err != nil {
		return ImportResult{}, err
	}
	for _, name := range sortedMarkdownNames(files) {
		existing, err := w.read(name)
		if err != nil {
			return ImportResult{}, err
		}
		switch {
		case existing == nil:
			result.Added++
		case bytes.Equal(existing, files[name]):
			result.Unchanged++
		default:
			return ImportResult{}, fmt.Errorf("Sage import conflicts with %s", name)
		}
	}
	var added []string
	rollback := func(cause error) error {
		var rollbackErrors []string
		for _, name := range added {
			if err := os.Remove(filepath.Join(w.Root, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, name)
			}
		}
		if err := restoreMaintenanceFile(filepath.Join(w.Root, "README.md"), originalIndex); err != nil {
			rollbackErrors = append(rollbackErrors, "README.md")
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf("%w (rollback failed for %s)", cause, strings.Join(rollbackErrors, ", "))
		}
		return cause
	}
	for _, name := range sortedMarkdownNames(files) {
		existing, err := w.read(name)
		if err != nil {
			return ImportResult{}, rollback(err)
		}
		if existing == nil {
			if err := w.write(name, files[name]); err != nil {
				return ImportResult{}, rollback(err)
			}
			added = append(added, name)
		}
	}
	if err := w.writeIndex(); err != nil {
		return ImportResult{}, rollback(err)
	}
	return result, nil
}

func restoreMaintenanceFile(path string, raw []byte) error {
	if raw == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return atomicReplace(path, raw)
}

func exactTopicFilename(topic string) (string, error) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if topic == "" || strings.ContainsAny(topic, `/\\`) {
		return "", errors.New("invalid Sage topic")
	}
	stem := strings.TrimSuffix(topic, ".md")
	name := SafeFilename(topic)
	if strings.TrimSuffix(name, ".md") != stem || name == "README.md" || strings.HasPrefix(name, "_") {
		return "", errors.New("invalid Sage topic")
	}
	return name, nil
}

func readMarkdownDirectory(root string) (map[string][]byte, error) {
	if err := validateDirectoryChain(root); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, errors.New("Sage Markdown directory unavailable")
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe Sage Markdown directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, errors.New("Sage Markdown directory unavailable")
	}
	files := make(map[string][]byte)
	total := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil, errors.New("unsafe Sage Markdown file")
		}
		if name != "README.md" && name != "_skipped.md" && SafeFilename(name) != name {
			return nil, errors.New("non-portable Sage Markdown filename")
		}
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, errors.New("Sage Markdown file unavailable")
		}
		if len(raw) > maxTopicBytes {
			return nil, errors.New("Sage topic exceeds size limit")
		}
		total += len(raw)
		if total > maxSnapshotBytes {
			return nil, errors.New("Sage Markdown snapshot exceeds size limit")
		}
		if name != "README.md" && !balancedFences(string(raw)) {
			return nil, errors.New("unclosed Sage topic code fence")
		}
		files[name] = raw
	}
	return files, nil
}

func validateDirectoryChain(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe Sage directory")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("Sage directory unavailable")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	contains := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return contains(left, right) || contains(right, left)
}

func sortedMarkdownNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func summarizeMarkdown(files map[string][]byte) RepairResult {
	result := RepairResult{}
	for name, raw := range files {
		if name == "README.md" || name == "_skipped.md" {
			continue
		}
		result.Topics++
		result.Entries += len(ParseMarkdown(string(raw)))
	}
	return result
}
