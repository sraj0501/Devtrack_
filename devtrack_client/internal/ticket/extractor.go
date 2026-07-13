package ticket

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultPatterns are the built-in regexes tried in order.
// They cover Jira-style IDs (PROJ-123), Azure DevOps (AB-7),
// GitHub/GitLab issue refs (#42), and short uppercase+digit fallback.
var DefaultPatterns = []string{
	`(?P<ticket>[A-Z][A-Z0-9]+-\d+)`, // Jira / ADO: PROJ-123, AB-7
	`#(\d+)`,                          // GitHub/GitLab issue: #42
	`(?P<ticket>[A-Z]+-\d+)`,          // Short fallback
}

// Extractor holds compiled patterns for ticket ID extraction.
type Extractor struct {
	patterns []*regexp.Regexp
}

// NewExtractor builds an Extractor. If customPattern is non-empty it is used
// as the sole pattern (returns error on bad regex). If empty, DefaultPatterns are used.
func NewExtractor(customPattern string) (*Extractor, error) {
	if customPattern != "" {
		re, err := regexp.Compile(customPattern)
		if err != nil {
			return nil, fmt.Errorf("ticket: invalid pattern %q: %w", customPattern, err)
		}
		return &Extractor{patterns: []*regexp.Regexp{re}}, nil
	}

	// Compile all DefaultPatterns.
	compiled := make([]*regexp.Regexp, 0, len(DefaultPatterns))
	for _, p := range DefaultPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			// DefaultPatterns are hardcoded and must compile; this is a programmer error.
			panic(fmt.Sprintf("ticket: DefaultPattern %q failed to compile: %v", p, err))
		}
		compiled = append(compiled, re)
	}
	return &Extractor{patterns: compiled}, nil
}

// DefaultExtractor is a convenience constructor using DefaultPatterns.
// Panics only if DefaultPatterns contains a malformed regex (compile-time invariant).
func DefaultExtractor() *Extractor {
	e, _ := NewExtractor("") // empty string always succeeds
	return e
}

// Extract returns the first ticket ID found in s, or "" if none.
// Named group "ticket" is preferred; falls back to capture group [1].
// Leading "#" is stripped from GitHub-style refs so the result is always "42" not "#42".
func (e *Extractor) Extract(s string) string {
	for _, re := range e.patterns {
		match := re.FindStringSubmatch(s)
		if match == nil {
			continue
		}

		var result string

		// Prefer named group "ticket" if it exists.
		idx := re.SubexpIndex("ticket")
		if idx >= 0 && idx < len(match) {
			result = match[idx]
		} else if len(match) > 1 {
			// Fall back to the first capture group.
			result = match[1]
		}

		if result == "" {
			continue
		}

		// Strip leading "#" from GitHub-style refs so the stored ID is "42" not "#42".
		result = strings.TrimPrefix(result, "#")
		return result
	}
	return ""
}
