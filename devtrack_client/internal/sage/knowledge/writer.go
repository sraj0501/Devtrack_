package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

// Serialize read/modify/replace operations across Writer instances in this
// process. Cross-process mutations also hold the Sage database writer lock.
var publicationMu sync.Mutex

const maxTopicBytes = 4 << 20
const indexStart = "<!-- sage:topics:start -->"
const indexEnd = "<!-- sage:topics:end -->"

// Writer publishes portable knowledge under a dedicated local directory.
// Topic publication precedes index publication; retry repairs the index without
// adding another entry. Callers must acknowledge a job only after Write succeeds.
type Writer struct {
	Root string
	// Recorder receives exact before/after bytes for optional safe local commits.
	Recorder interface{ Record(string, []byte, []byte) }
	// replace is an internal fault-injection seam, never model supplied.
	replace func(string, []byte) error
}

// Write reuses an existing signature anywhere in the knowledge directory,
// preserving user edits and corrected file placement. New entries append to
// their section in stable document order, matching the pinned reference.
// When topic is a binary, existing entries and remembered routes select its
// destination before falling back to the binary's own filename.
func (w Writer) Write(topic, signature string, draft distill.Draft) (string, error) {
	body, err := RenderDraft(draft, signature)
	if err != nil {
		return "", err
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return "", err
	}
	if _, err := w.recoverMerge(); err != nil {
		return "", err
	}
	if err := w.ensureGitignore(); err != nil {
		return "", err
	}
	files, err := w.topics()
	if err != nil {
		return "", err
	}
	for _, name := range files {
		text, err := w.read(name)
		if err != nil {
			return "", err
		}
		for _, entry := range ParseMarkdown(string(text)) {
			for _, sig := range entry.Signatures {
				if sig == signature {
					return name, w.writeIndex()
				}
			}
		}
	}
	name := SafeFilename(topic)
	routes, err := w.routes()
	if err != nil {
		return "", err
	}
	for _, route := range routes {
		if route.Binary == strings.ToLower(strings.TrimSpace(topic)) {
			name = route.Filename
			topic = route.Title
			break
		}
	}
	text, err := w.read(name)
	if err != nil {
		return "", err
	}
	if len(text) == 0 {
		title := prose(strings.TrimSuffix(strings.TrimSpace(topic), ".md"))
		if title == "" {
			title = "Misc"
		}
		text = []byte(fmt.Sprintf("# %s\n\n> Auto-maintained by DevTrack Sage. Signature comments are dedupe keys.\n", title))
	}
	if !balancedFences(string(text)) {
		return "", errors.New("unclosed Sage topic code fence")
	}
	section := prose(strings.Trim(strings.TrimSpace(draft.Section), "# "))
	if section == "" {
		section = "Other"
	}
	updated := insertMarkdown(string(text), section, body)
	if err := w.write(name, []byte(updated)); err != nil {
		return "", err
	}
	return name, w.writeIndex()
}

func balancedFences(text string) bool {
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if fence != "" {
			if closesFence(line, fence) {
				fence = ""
			}
		} else {
			fence = openingFence(line)
		}
	}
	return fence == ""
}

// AddSignature attaches a variant to an existing entry without changing prose.
// It returns false when already present or the target signature is absent.
// Cross-process callers must hold the shared Sage database writer lock.
func (w Writer) AddSignature(topic, existing, variant string) (bool, error) {
	if !validSignature.MatchString(existing) || !validSignature.MatchString(variant) {
		return false, errors.New("invalid Sage signature")
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return false, err
	}
	if _, err := w.recoverMerge(); err != nil {
		return false, err
	}
	name := SafeFilename(topic)
	raw, err := w.read(name)
	if err != nil {
		return false, err
	}
	entries := ParseMarkdown(string(raw))
	for _, entry := range entries {
		for _, sig := range entry.Signatures {
			if sig == variant {
				return false, nil
			}
		}
	}
	for _, entry := range entries {
		for _, sig := range entry.Signatures {
			if sig != existing {
				continue
			}
			lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
			lines = append(lines[:entry.SignatureEnd], append([]string{"<!-- sig: " + variant + " -->"}, lines[entry.SignatureEnd:]...)...)
			return true, w.write(name, []byte(strings.Join(lines, "\n")))
		}
	}
	return false, nil
}

func insertMarkdown(text, section, body string) string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	target, nearTarget, end, fence := -1, -1, len(lines), ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if fence != "" {
			if closesFence(trim, fence) {
				fence = ""
			}
			continue
		}
		if f := openingFence(trim); f != "" {
			fence = f
			continue
		}
		if strings.HasPrefix(line, "## ") {
			if target >= 0 {
				end = i
				break
			}
			existing := strings.TrimSpace(line[3:])
			if strings.EqualFold(existing, section) {
				target = i
			} else if nearTarget < 0 && nearSection(existing, section) {
				nearTarget = i
			}
		}
	}
	if target < 0 && nearTarget >= 0 {
		target, end, fence = nearTarget, len(lines), ""
		for i := target + 1; i < len(lines); i++ {
			trim := strings.TrimSpace(lines[i])
			if fence != "" {
				if closesFence(trim, fence) {
					fence = ""
				}
				continue
			}
			if f := openingFence(trim); f != "" {
				fence = f
				continue
			}
			if strings.HasPrefix(lines[i], "## ") {
				end = i
				break
			}
		}
	}
	if target < 0 {
		return strings.Join(lines, "\n") + "\n\n## " + section + "\n\n" + body
	}
	for end > target+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	out := strings.Join(lines[:end], "\n") + "\n\n" + strings.TrimRight(body, "\n") + "\n"
	if end < len(lines) {
		out += "\n" + strings.TrimLeft(strings.Join(lines[end:], "\n"), "\n") + "\n"
	}
	return out
}

func nearSection(left, right string) bool {
	a, b := strings.Fields(strings.ToLower(prose(left))), strings.Fields(strings.ToLower(prose(right)))
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	difference := 0
	for i := range a {
		x, y := strings.Trim(a[i], "-_.:,;!?"), strings.Trim(b[i], "-_.:,;!?")
		if x == y || strings.TrimSuffix(x, "s") == strings.TrimSuffix(y, "s") {
			continue
		}
		if oneEditApart(x, y) {
			difference++
			continue
		}
		return false
	}
	return difference <= 1
}

func oneEditApart(left, right string) bool {
	if left == right || len(left)-len(right) > 1 || len(right)-len(left) > 1 {
		return left == right
	}
	i, j, edits := 0, 0, 0
	for i < len(left) && j < len(right) {
		if left[i] == right[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		switch {
		case len(left) > len(right):
			i++
		case len(right) > len(left):
			j++
		default:
			i++
			j++
		}
	}
	if i < len(left) || j < len(right) {
		edits++
	}
	return edits == 1
}

func (w Writer) prepare() error {
	if strings.TrimSpace(w.Root) == "" {
		return errors.New("Sage knowledge directory is required")
	}
	abs, err := filepath.Abs(w.Root)
	if err != nil {
		return err
	}
	// Reject pre-existing links at every component, including directory junctions.
	for p := abs; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return errors.New("unsafe Sage knowledge directory")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return err
	}
	return nil
}

func (w Writer) ensureGitignore() error {
	const block = "# DevTrack Sage runtime\n.routes.json\n.merge.json\n.sage-*.tmp\n"
	raw, err := w.read(".gitignore")
	if err != nil {
		return err
	}
	text := string(raw)
	missing := make([]string, 0, 3)
	for _, line := range []string{".routes.json", ".merge.json", ".sage-*.tmp"} {
		found := false
		for _, existing := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(existing) == line {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if len(raw) == 0 {
		return w.write(".gitignore", []byte(block))
	}
	updated := strings.TrimRight(text, "\r\n") + "\n\n# DevTrack Sage runtime\n" + strings.Join(missing, "\n") + "\n"
	return w.write(".gitignore", []byte(updated))
}

func (w Writer) topics() ([]string, error) {
	entries, err := os.ReadDir(w.Root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.EqualFold(name, "README.md") || strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, errors.New("unsafe Sage topic file")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (w Writer) read(name string) ([]byte, error) {
	path := filepath.Join(w.Root, name)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("unsafe Sage knowledge file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxTopicBytes+1))
	if len(raw) > maxTopicBytes {
		return nil, errors.New("Sage topic exceeds size limit")
	}
	return raw, err
}

func (w Writer) write(name string, raw []byte) error {
	if len(raw) > maxTopicBytes {
		return errors.New("Sage topic exceeds size limit")
	}
	old, err := w.read(name)
	if err != nil {
		return err
	}
	if bytes.Equal(old, raw) {
		return nil
	}
	if w.replace != nil {
		err = w.replace(filepath.Join(w.Root, name), raw)
	} else {
		err = atomicReplace(filepath.Join(w.Root, name), raw)
	}
	if err == nil && w.Recorder != nil {
		w.Recorder.Record(filepath.Join(w.Root, name), old, raw)
	}
	return err
}

func (w Writer) remove(name string) error {
	old, err := w.read(name)
	if err != nil || old == nil {
		return err
	}
	if err := os.Remove(filepath.Join(w.Root, name)); err != nil {
		return err
	}
	if w.Recorder != nil {
		w.Recorder.Record(filepath.Join(w.Root, name), old, nil)
	}
	return nil
}

func (w Writer) writeIndex() error {
	names, err := w.topics()
	if err != nil {
		return err
	}
	var block strings.Builder
	block.WriteString(indexStart + "\n\n| Topic | Entries |\n| --- | ---: |\n")
	for _, name := range names {
		raw, err := w.read(name)
		if err != nil {
			return err
		}
		// Existing names may contain Markdown syntax; never emit unsafe links.
		if SafeFilename(name) != name {
			continue
		}
		fmt.Fprintf(&block, "| [%s](%s) | %d |\n", strings.TrimSuffix(name, ".md"), name, len(ParseMarkdown(string(raw))))
	}
	block.WriteString("\n" + indexEnd)
	raw, err := w.read("README.md")
	if err != nil {
		return err
	}
	text := string(raw)
	start, end := strings.Index(text, indexStart), strings.Index(text, indexEnd)
	if start < 0 && end < 0 {
		if text == "" {
			text = "# Sage knowledge\n"
		}
		text = strings.TrimRight(text, "\n") + "\n\n" + block.String() + "\n"
	} else {
		if start < 0 || end < start || strings.Count(text, indexStart) != 1 || strings.Count(text, indexEnd) != 1 {
			return errors.New("malformed Sage topic index markers")
		}
		text = text[:start] + block.String() + text[end+len(indexEnd):]
	}
	return w.write("README.md", []byte(text))
}

func atomicReplace(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sage-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmp.Name(), path)
}
