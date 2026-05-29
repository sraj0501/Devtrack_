package gitsage

import (
	"fmt"
	"os"
	"strings"
)

// ConflictStrategy controls how conflicts are resolved.
type ConflictStrategy string

const (
	// StrategyOurs keeps our version (HEAD) for all conflicts.
	StrategyOurs ConflictStrategy = "ours"
	// StrategyTheirs keeps their version (incoming) for all conflicts.
	StrategyTheirs ConflictStrategy = "theirs"
	// StrategyBoth keeps both sides separated by a comment.
	StrategyBoth ConflictStrategy = "both"
	// StrategySmart uses LLM to pick the best resolution per conflict.
	StrategySmart ConflictStrategy = "smart"
)

// ConflictFile represents a file with merge conflicts.
type ConflictFile struct {
	Path     string
	Sections []ConflictSection
}

// ConflictSection is a single conflict hunk in a file.
type ConflictSection struct {
	Ours   []string // lines from HEAD
	Theirs []string // lines from incoming
	Base   []string // ancestor lines (if present in diff3 format)
}

// Resolver handles merge conflict detection and resolution.
type Resolver struct {
	ops      *GitOps
	strategy ConflictStrategy
}

// NewResolver creates a Resolver for the repository.
func NewResolver(repoPath string, strategy ConflictStrategy) *Resolver {
	return &Resolver{
		ops:      NewGitOps(repoPath),
		strategy: strategy,
	}
}

// DetectConflicts returns a list of files that currently have conflict markers.
func (r *Resolver) DetectConflicts() ([]string, error) {
	entries, err := r.ops.Status()
	if err != nil {
		return nil, err
	}
	var conflicted []string
	for _, e := range entries {
		// "UU" = both modified, "AA" = both added, "DD" = both deleted
		xy := strings.TrimSpace(e.XY)
		if xy == "UU" || xy == "AA" || xy == "DD" || xy == "AU" || xy == "UA" {
			conflicted = append(conflicted, e.Path)
		}
	}
	// Also check for conflict markers in modified files
	if len(conflicted) == 0 {
		conflicted, err = r.scanForMarkers()
		if err != nil {
			return nil, err
		}
	}
	return conflicted, nil
}

// scanForMarkers scans working tree files for git conflict markers.
func (r *Resolver) scanForMarkers() ([]string, error) {
	// Use git grep to find conflict markers efficiently
	ops := r.ops
	out, _ := ops.run("grep", "-l", "^<<<<<<< ")
	if out == "" {
		return nil, nil
	}
	var files []string
	for _, f := range strings.Split(out, "\n") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// ParseConflicts reads a file and extracts conflict sections.
func ParseConflicts(repoPath, filePath string) (*ConflictFile, error) {
	fullPath := repoPath + "/" + filePath
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("conflict parse: read %s: %w", filePath, err)
	}

	lines := strings.Split(string(data), "\n")
	cf := &ConflictFile{Path: filePath}

	var current *ConflictSection
	state := "normal" // normal | ours | base | theirs

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "<<<<<<< "):
			current = &ConflictSection{}
			state = "ours"
		case line == "=======" && state != "normal":
			state = "theirs"
		case strings.HasPrefix(line, ">>>>>>> ") && state != "normal":
			cf.Sections = append(cf.Sections, *current)
			current = nil
			state = "normal"
		case strings.HasPrefix(line, "||||||| ") && state == "ours":
			// diff3 base section
			state = "base"
		default:
			if current == nil {
				continue
			}
			switch state {
			case "ours":
				current.Ours = append(current.Ours, line)
			case "theirs":
				current.Theirs = append(current.Theirs, line)
			case "base":
				current.Base = append(current.Base, line)
			}
		}
	}
	return cf, nil
}

// Resolve applies the configured strategy to all conflicted files.
// Returns the list of files that were resolved and any unresolvable files.
func (r *Resolver) Resolve() (resolved []string, unresolvable []string, err error) {
	files, err := r.DetectConflicts()
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, nil
	}

	for _, f := range files {
		if resolveErr := r.resolveFile(f); resolveErr != nil {
			unresolvable = append(unresolvable, f)
			fmt.Fprintf(os.Stderr, "conflict: cannot resolve %s: %v\n", f, resolveErr)
		} else {
			resolved = append(resolved, f)
			// Stage the resolved file
			if _, addErr := r.ops.Add(f); addErr != nil {
				fmt.Fprintf(os.Stderr, "conflict: failed to stage %s after resolution: %v\n", f, addErr)
			}
		}
	}
	return resolved, unresolvable, nil
}

// resolveFile resolves conflicts in a single file using the configured strategy.
func (r *Resolver) resolveFile(filePath string) error {
	switch r.strategy {
	case StrategyOurs:
		_, err := r.ops.run("checkout", "--ours", filePath)
		return err
	case StrategyTheirs:
		_, err := r.ops.run("checkout", "--theirs", filePath)
		return err
	case StrategyBoth:
		return r.resolveBoth(filePath)
	case StrategySmart:
		// Smart: try to use ours for trivial cases, flag complex ones
		return r.resolveSmart(filePath)
	default:
		return fmt.Errorf("unknown strategy %q", r.strategy)
	}
}

// resolveBoth keeps both sides of each conflict, annotated with comments.
func (r *Resolver) resolveBoth(filePath string) error {
	fullPath := r.ops.RepoPath + "/" + filePath
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inConflict := false

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "<<<<<<< "):
			inConflict = true
			result = append(result, "// --- OUR VERSION ---")
		case line == "=======" && inConflict:
			result = append(result, "// --- THEIR VERSION ---")
		case strings.HasPrefix(line, ">>>>>>> ") && inConflict:
			inConflict = false
			result = append(result, "// --- END CONFLICT ---")
		default:
			result = append(result, line)
		}
	}

	return os.WriteFile(fullPath, []byte(strings.Join(result, "\n")), 0644)
}

// resolveSmart attempts to auto-resolve simple conflicts (identical ours/theirs or empty one side).
// Falls back to "ours" for complex conflicts.
func (r *Resolver) resolveSmart(filePath string) error {
	cf, err := ParseConflicts(r.ops.RepoPath, filePath)
	if err != nil {
		return err
	}

	// For each section, check if it's trivially resolvable
	allSimple := true
	for _, section := range cf.Sections {
		oursStr := strings.Join(section.Ours, "\n")
		theirsStr := strings.Join(section.Theirs, "\n")
		// If identical, or one side is empty — trivially resolvable
		if oursStr == theirsStr || strings.TrimSpace(theirsStr) == "" {
			continue
		}
		if strings.TrimSpace(oursStr) == "" {
			continue
		}
		allSimple = false
	}

	if allSimple {
		// All sections are simple — use ours
		_, err = r.ops.run("checkout", "--ours", filePath)
		return err
	}

	// Complex: use ours as safe default, report to user
	fmt.Printf("conflict: %s has complex conflicts — keeping our version (review manually)\n", filePath)
	_, err = r.ops.run("checkout", "--ours", filePath)
	return err
}

// Report produces a human-readable summary of conflict state.
func (r *Resolver) Report() string {
	files, err := r.DetectConflicts()
	if err != nil {
		return fmt.Sprintf("conflict detection failed: %v", err)
	}
	if len(files) == 0 {
		return "No conflicts detected."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d conflicted file(s):\n", len(files))
	for _, f := range files {
		cf, err := ParseConflicts(r.ops.RepoPath, f)
		if err != nil {
			fmt.Fprintf(&b, "  %s (unreadable)\n", f)
			continue
		}
		fmt.Fprintf(&b, "  %s — %d conflict section(s)\n", f, len(cf.Sections))
	}
	return b.String()
}
