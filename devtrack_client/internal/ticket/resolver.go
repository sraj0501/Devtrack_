package ticket

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DefaultKinds are the branch kinds accepted by the canonical convention.
var DefaultKinds = []string{
	"feature", "feat", "fix", "bugfix", "hotfix", "chore", "docs", "refactor", "test",
}

type Source string

const (
	SourceNone          Source = "none"
	SourceBranch        Source = "branch"
	SourceMergeBranch   Source = "merge_branch"
	SourceCommitPrefix  Source = "commit_prefix"
	SourceCommitTrailer Source = "commit_trailer"
	SourceActiveTicket  Source = "active_ticket"
)

type State string

const (
	StateLinked   State = "linked"
	StateUnlinked State = "unlinked"
	StateConflict State = "conflict"
)

// Evidence is one deterministic ticket signal considered by Resolve.
type Evidence struct {
	TicketID string
	Source   Source
}

// Result is the explainable deterministic mapping for one commit.
type Result struct {
	TicketID   string
	Source     Source
	State      State
	Confidence float64
	Conflict   bool
	Evidence   []Evidence
	Reason     string
}

type ResolverConfig struct {
	TicketKey     string
	Kinds         []string
	BranchPattern string
}

type ResolveInput struct {
	Branch        string
	CommitMessage string
	ActiveTicket  string
	IsMerge       bool
}

// Resolver applies the deterministic TASK-160 precedence contract.
type Resolver struct {
	branchPattern *regexp.Regexp
	ticketPattern *regexp.Regexp
}

// NewResolver creates a resolver for one workspace. BranchPattern, when set,
// must be fully anchored and contain exactly one named "ticket" capture.
func NewResolver(cfg ResolverConfig) (*Resolver, error) {
	key := strings.TrimSpace(cfg.TicketKey)
	if key != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9]*$`).MatchString(key) {
		return nil, fmt.Errorf("ticket: invalid ticket key %q", cfg.TicketKey)
	}

	kinds := cfg.Kinds
	if len(kinds) == 0 {
		kinds = DefaultKinds
	}
	cleanKinds := make([]string, 0, len(kinds))
	seenKinds := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		kind = strings.TrimSpace(kind)
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(kind) {
			return nil, fmt.Errorf("ticket: invalid branch kind %q", kind)
		}
		if _, exists := seenKinds[kind]; exists {
			continue
		}
		seenKinds[kind] = struct{}{}
		cleanKinds = append(cleanKinds, regexp.QuoteMeta(kind))
	}
	if len(cleanKinds) == 0 {
		return nil, fmt.Errorf("ticket: at least one branch kind is required")
	}

	branchExpr := strings.TrimSpace(cfg.BranchPattern)
	if branchExpr == "" {
		keyExpr := `[A-Z][A-Z0-9]*`
		if key != "" {
			keyExpr = regexp.QuoteMeta(key)
		}
		branchExpr = `^(?:` + strings.Join(cleanKinds, "|") + `)/(?P<ticket>` + keyExpr + `-[1-9][0-9]*)-[a-z0-9]+(?:-[a-z0-9]+)*$`
	}
	branchPattern, err := compileBranchPattern(branchExpr)
	if err != nil {
		return nil, err
	}

	ticketKeyExpr := `[A-Z][A-Z0-9]*`
	if key != "" {
		ticketKeyExpr = regexp.QuoteMeta(key)
	}
	return &Resolver{
		branchPattern: branchPattern,
		ticketPattern: regexp.MustCompile(`^` + ticketKeyExpr + `-[1-9][0-9]*$`),
	}, nil
}

func compileBranchPattern(pattern string) (*regexp.Regexp, error) {
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		return nil, fmt.Errorf("ticket: branch pattern must be fully anchored")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("ticket: invalid branch pattern %q: %w", pattern, err)
	}
	count := 0
	for _, name := range re.SubexpNames() {
		if name == "ticket" {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("ticket: branch pattern must contain exactly one named ticket capture")
	}
	return re, nil
}

func (r *Resolver) branchTicket(branch string) string {
	match := r.branchPattern.FindStringSubmatch(strings.TrimSpace(branch))
	if match == nil {
		return ""
	}
	idx := r.branchPattern.SubexpIndex("ticket")
	if idx <= 0 || idx >= len(match) || !r.ticketPattern.MatchString(match[idx]) {
		return ""
	}
	return match[idx]
}

func (r *Resolver) explicitMessageEvidence(message string) []Evidence {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil
	}

	var evidence []Evidence
	first := strings.TrimSpace(lines[0])
	if colon := strings.Index(first, ":"); colon > 0 {
		candidate := strings.TrimSpace(first[:colon])
		if r.ticketPattern.MatchString(candidate) {
			evidence = append(evidence, Evidence{TicketID: candidate, Source: SourceCommitPrefix})
		}
	}

	for _, line := range lines[1:] {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Refs") {
			continue
		}
		candidate := strings.TrimSpace(parts[1])
		if r.ticketPattern.MatchString(candidate) {
			evidence = append(evidence, Evidence{TicketID: candidate, Source: SourceCommitTrailer})
		}
	}
	return evidence
}

func (r *Resolver) mergeEvidence(message string) []Evidence {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^Merge branch '([^']+)'`),
		regexp.MustCompile(`(?m)^Merge pull request #[0-9]+ from [^/]+/(\S+)`),
	}
	var evidence []Evidence
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(message, -1) {
			if len(match) < 2 {
				continue
			}
			if id := r.branchTicket(strings.TrimSpace(match[1])); id != "" {
				evidence = append(evidence, Evidence{TicketID: id, Source: SourceMergeBranch})
			}
		}
	}
	return evidence
}

// Resolve applies canonical branch, explicit message, explicit active ticket,
// then unlinked precedence. Incidental prose and prior mappings are ignored.
func (r *Resolver) Resolve(input ResolveInput) Result {
	var branchEvidence []Evidence
	if id := r.branchTicket(input.Branch); id != "" {
		branchEvidence = append(branchEvidence, Evidence{TicketID: id, Source: SourceBranch})
	}
	if input.IsMerge {
		branchEvidence = append(branchEvidence, r.mergeEvidence(input.CommitMessage)...)
	}
	messageEvidence := r.explicitMessageEvidence(input.CommitMessage)
	var activeEvidence []Evidence
	if active := strings.TrimSpace(input.ActiveTicket); r.ticketPattern.MatchString(active) {
		activeEvidence = append(activeEvidence, Evidence{TicketID: active, Source: SourceActiveTicket})
	}

	all := append([]Evidence{}, branchEvidence...)
	all = append(all, messageEvidence...)
	all = append(all, activeEvidence...)

	levels := [][]Evidence{branchEvidence, messageEvidence, activeEvidence}
	confidences := []float64{1.0, 0.95, 1.0}
	for level, candidates := range levels {
		if len(candidates) == 0 {
			continue
		}
		ids := distinctIDs(candidates)
		if len(ids) != 1 {
			return Result{State: StateUnlinked, Source: SourceNone, Conflict: true, Evidence: all,
				Reason: "multiple ticket references at the winning priority"}
		}
		winner := ids[0]
		conflict := false
		for _, lower := range levels[level+1:] {
			for _, item := range lower {
				if item.TicketID != winner {
					conflict = true
				}
			}
		}
		state := StateLinked
		reason := ""
		if conflict {
			state = StateConflict
			reason = "lower-priority ticket evidence contradicts the authoritative mapping"
		}
		return Result{TicketID: winner, Source: candidates[0].Source, State: state,
			Confidence: confidences[level], Conflict: conflict, Evidence: all, Reason: reason}
	}

	return Result{State: StateUnlinked, Source: SourceNone, Evidence: all,
		Reason: "no deterministic ticket evidence"}
}

func distinctIDs(evidence []Evidence) []string {
	set := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		set[item.TicketID] = struct{}{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ExternalID converts a canonical branch-safe reference into the provider's
// native identifier without weakening the canonical convention.
func ExternalID(canonicalRef, platform string) string {
	canonicalRef = strings.TrimSpace(canonicalRef)
	if canonicalRef == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "github", "gitlab", "azure", "azure_devops", "ado":
		if dash := strings.LastIndex(canonicalRef, "-"); dash >= 0 && dash+1 < len(canonicalRef) {
			return canonicalRef[dash+1:]
		}
	}
	return canonicalRef
}
