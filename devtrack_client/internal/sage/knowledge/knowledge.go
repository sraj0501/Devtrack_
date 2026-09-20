// Package knowledge defines deterministic, model-free Sage knowledge records.
package knowledge

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var queryToken = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._-]*`)

// Entry is the stable, privacy-minimized view returned by Sage search.
type Entry struct {
	ID              int64
	Signature       string
	Topic           string
	Command         string
	ProjectID       string
	UseCount        int
	SuccessCount    int
	FailureCount    int
	FirstSeen       time.Time
	LastSeen        time.Time
	SourceHarnesses []string
}

// Topic summarizes a command family without exposing event payloads.
type Topic struct {
	Name     string
	Entries  int
	Uses     int
	LastSeen time.Time
}

// TopicForSignature deterministically assigns a normalized signature to its
// command family. Signatures are contract-validated before reaching here.
func TopicForSignature(signature string) string {
	fields := strings.Fields(strings.ToLower(signature))
	if len(fields) == 0 {
		return "other"
	}
	return fields[0]
}

// SearchExpression converts user text into a literal FTS5 AND query. It does
// not pass operators or column selectors through to SQLite.
func SearchExpression(query string) string {
	tokens := queryToken.FindAllString(strings.ToLower(query), -1)
	for i, token := range tokens {
		tokens[i] = `"` + token + `"`
	}
	return strings.Join(tokens, " AND ")
}

// RenderEntries produces byte-stable Markdown suitable for the CLI and later
// read-only integrations.
func RenderEntries(entries []Entry) string {
	if len(entries) == 0 {
		return "No Sage knowledge matched.\n"
	}
	var out strings.Builder
	for i, entry := range entries {
		if i > 0 {
			out.WriteByte('\n')
		}
		sources := append([]string(nil), entry.SourceHarnesses...)
		sort.Strings(sources)
		fmt.Fprintf(&out, "## %s\n\n", entry.Signature)
		fmt.Fprintf(&out, "- Topic: `%s`\n", entry.Topic)
		fmt.Fprintf(&out, "- Seen: %d (%d succeeded, %d failed)\n", entry.UseCount, entry.SuccessCount, entry.FailureCount)
		if len(sources) > 0 {
			fmt.Fprintf(&out, "- Sources: `%s`\n", strings.Join(sources, "`, `"))
		}
		fmt.Fprintf(&out, "- Last used: %s\n\n", entry.LastSeen.UTC().Format(time.RFC3339))
		fmt.Fprintf(&out, "    %s\n", entry.Command)
	}
	return out.String()
}

// RenderTopics produces a deterministic compact topic index.
func RenderTopics(topics []Topic) string {
	if len(topics) == 0 {
		return "No Sage topics yet.\n"
	}
	var out strings.Builder
	out.WriteString("# Sage topics\n\n")
	for _, topic := range topics {
		fmt.Fprintf(&out, "- `%s`: %d entries, %d uses (last %s)\n",
			topic.Name, topic.Entries, topic.Uses, topic.LastSeen.UTC().Format(time.RFC3339))
	}
	return out.String()
}
