package ticket

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
)

// Contract is workspace-owned identity configuration. Pattern, when present,
// describes a whole branch and captures one canonical reference named ticket.
type Contract struct {
	Key      string `json:"key"`
	Provider string `json:"provider"`
	Pattern  string `json:"pattern,omitempty"`
}

type Resolution struct {
	TicketID   string   `json:"ticket_id"`
	ExternalID string   `json:"external_id"`
	Source     string   `json:"source"`
	Confidence float64  `json:"confidence"`
	State      string   `json:"state"`
	Branch     string   `json:"branch"`
	Conflict   bool     `json:"conflict"`
	Reason     string   `json:"reason,omitempty"`
	References []string `json:"references,omitempty"`
}

type Evidence struct {
	Branch         string
	Message        string
	ActiveTicket   string // Explicit state, already scoped to workspace and repository.
	MergeToDefault bool
}

type Resolver struct {
	contract Contract
	branch   *regexp.Regexp
	ref      *regexp.Regexp
}

const DefaultKinds = "feature|feat|fix|bugfix|hotfix|chore|docs|refactor|test"

var namespace = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
var explicitPrefix = regexp.MustCompile(`^([A-Z][A-Z0-9]*-[1-9][0-9]*):(?:\s|$)`)
var trailerLine = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:[ \t]*.*$`)
var mergeQuotedBranch = regexp.MustCompile(`'([^']+)'`)

func NewResolver(c Contract) (*Resolver, error) {
	if !namespace.MatchString(c.Key) {
		return nil, fmt.Errorf("ticket_key must be an uppercase namespace")
	}
	switch c.Provider {
	case "", "none", "jira", "github", "gitlab", "azure":
	default:
		return nil, fmt.Errorf("unsupported ticket provider %q", c.Provider)
	}
	pattern := c.Pattern
	if pattern == "" {
		pattern = `^(?:` + DefaultKinds + `)/(?P<ticket>` + c.Key + `-[1-9][0-9]*)-[a-z0-9]+(?:-[a-z0-9]+)*$`
	}
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, fmt.Errorf("ticket_pattern: %w", err)
	}
	// Checking the syntax tree rejects misleading ^a|b$ and multiline anchors.
	for parsed.Op == syntax.OpCapture {
		parsed = parsed.Sub[0]
	}
	if parsed.Op != syntax.OpConcat || len(parsed.Sub) < 3 || parsed.Sub[0].Op != syntax.OpBeginText || parsed.Sub[len(parsed.Sub)-1].Op != syntax.OpEndText {
		return nil, fmt.Errorf("ticket_pattern must anchor the entire branch with ^ and $")
	}
	branch, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("ticket_pattern: %w", err)
	}
	count := 0
	for _, name := range branch.SubexpNames() {
		if name == "ticket" {
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("ticket_pattern must contain exactly one named ticket capture")
	}
	return &Resolver{contract: c, branch: branch, ref: regexp.MustCompile(`^` + c.Key + `-[1-9][0-9]*$`)}, nil
}

func (r *Resolver) ValidReference(ref string) bool { return r.ref.MatchString(ref) }

func (r *Resolver) BranchReference(branch string) string {
	m := r.branch.FindStringSubmatch(branch)
	if m == nil || m[0] != branch {
		return ""
	}
	ref := m[r.branch.SubexpIndex("ticket")]
	if !r.ValidReference(ref) {
		return ""
	}
	return ref
}

func (r *Resolver) ExternalID(ref string) string {
	if !r.ValidReference(ref) {
		return ""
	}
	switch r.contract.Provider {
	case "github", "gitlab", "azure":
		return strings.TrimPrefix(ref, r.contract.Key+"-")
	default:
		return ref
	}
}

// ExplicitReferences ignores incidental prose. Refs trailers must belong to the
// final trailer paragraph, not a quoted line somewhere in the message body.
func (r *Resolver) ExplicitReferences(message string) []string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(message, "\r\n", "\n"), "\n"), "\n")
	refs := make(map[string]bool)
	if m := explicitPrefix.FindStringSubmatch(lines[0]); m != nil && r.ValidReference(m[1]) {
		refs[m[1]] = true
	}
	start := len(lines) - 1
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	valid := true
	for _, line := range lines[start:] {
		if !trailerLine.MatchString(line) {
			valid = false
			break
		}
	}
	if valid {
		for _, line := range lines[start:] {
			key, value, _ := strings.Cut(line, ":")
			if !strings.EqualFold(key, "Refs") {
				continue
			}
			for _, ref := range strings.FieldsFunc(value, func(c rune) bool { return c == ',' || c == ' ' || c == '\t' }) {
				if r.ValidReference(ref) {
					refs[ref] = true
				}
			}
		}
	}
	return sortedRefs(refs)
}

func sortedRefs(refs map[string]bool) []string {
	result := make([]string, 0, len(refs))
	for ref := range refs {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result
}

func (r *Resolver) mergeReferences(message string) []string {
	subject := strings.SplitN(message, "\n", 2)[0]
	refs := make(map[string]bool)
	if strings.HasPrefix(subject, "Merge branch ") || strings.HasPrefix(subject, "Merge remote-tracking branch ") {
		for _, m := range mergeQuotedBranch.FindAllStringSubmatch(subject, -1) {
			branch := strings.TrimPrefix(m[1], "origin/")
			if ref := r.BranchReference(branch); ref != "" {
				refs[ref] = true
			}
		}
	} else if strings.HasPrefix(subject, "Merge pull request #") {
		_, branch, found := strings.Cut(subject, " from ")
		if found {
			// GitHub's merge subject is owner/canonical-branch.
			_, branch, found = strings.Cut(branch, "/")
			if found {
				if ref := r.BranchReference(branch); ref != "" {
					refs[ref] = true
				}
			}
		}
	}
	return sortedRefs(refs)
}

func (r *Resolver) Resolve(e Evidence) Resolution {
	out := Resolution{State: "unlinked", Source: "none", Branch: e.Branch, Reason: "no deterministic ticket evidence"}
	branch := r.BranchReference(e.Branch)
	first := []string{}
	firstSource := "branch"
	if branch != "" {
		first = append(first, branch)
	} else if e.MergeToDefault {
		first = r.mergeReferences(e.Message)
		firstSource = "merge_branch"
	}
	explicit := r.ExplicitReferences(e.Message)
	active := []string{}
	if r.ValidReference(e.ActiveTicket) {
		active = append(active, e.ActiveTicket)
	}
	all := make(map[string]bool)
	for _, group := range [][]string{first, explicit, active} {
		for _, ref := range group {
			all[ref] = true
		}
	}
	out.References = sortedRefs(all)
	groups := [][]string{first, explicit, active}
	sources := []string{firstSource, "commit_reference", "active_override"}
	for i, group := range groups {
		if len(group) == 0 {
			continue
		}
		out.Source = sources[i]
		if len(group) > 1 {
			out.Conflict, out.Reason = true, "ambiguous references at the winning priority"
			return out // Never fall through to a weaker signal.
		}
		out.TicketID, out.ExternalID = group[0], r.ExternalID(group[0])
		out.State, out.Confidence, out.Reason = "linked", 1, ""
		if len(all) > 1 {
			out.State, out.Conflict, out.Reason = "conflict", true, "contradictory lower-priority evidence"
		}
		return out
	}
	return out
}
