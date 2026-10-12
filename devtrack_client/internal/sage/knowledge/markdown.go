package knowledge

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

var signatureLine = regexp.MustCompile(`^<!--\s*sig:\s*([0-9a-f]{6,64})\s*-->$`)
var validSignature = regexp.MustCompile(`^[0-9a-f]{6,64}$`)
var filenameUnsafe = regexp.MustCompile(`[^a-z0-9._-]`)
var windowsDevice = regexp.MustCompile(`^(con|prn|aux|nul|com[0-9]|lpt[0-9])($|\.)`)

// SafeFilename maps a topic to one portable, relative Markdown filename.
// Reserved index/skip names and Windows device names cannot become topics.
func SafeFilename(name string) string {
	stem := filenameUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	stem = strings.Trim(strings.TrimSuffix(stem, ".md"), "-.")
	if stem == "" {
		stem = "misc"
	}
	if len(stem) > 100 {
		stem = strings.TrimRight(stem[:100], ".-")
	}
	if windowsDevice.MatchString(stem) || stem == "readme" || strings.HasPrefix(stem, "_") {
		stem = "topic-" + stem
	}
	return stem + ".md"
}

// MarkdownEntry describes an existing entry without reformatting its prose.
// Start, End and SignatureEnd are zero-based line offsets for lossless insertion.
type MarkdownEntry struct {
	Heading, Section         string
	Signatures, Commands     []string
	Start, End, SignatureEnd int
}

// ParseMarkdown ignores headings and signature-like text inside fenced code.
func ParseMarkdown(text string) []MarkdownEntry {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var entries []MarkdownEntry
	section, fence := "Other", ""
	active, firstCode, readingCode, sigs := -1, false, false, false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if fence != "" {
			if closesFence(trim, fence) {
				fence = ""
				readingCode = false
			} else if readingCode && trim != "" {
				entries[active].Commands = append(entries[active].Commands, trim)
			}
			continue
		}
		if f := openingFence(trim); f != "" {
			fence = f
			readingCode = active >= 0 && !firstCode
			if readingCode {
				firstCode = true
			}
			sigs = false
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			if active >= 0 {
				entries[active].End = i
				active = -1
			}
			if strings.HasPrefix(line, "## ") {
				section = strings.TrimSpace(line[3:])
			}
		}
		if strings.HasPrefix(line, "### ") {
			if active >= 0 {
				entries[active].End = i
			}
			entries = append(entries, MarkdownEntry{Heading: strings.TrimSpace(line[4:]), Section: section, Start: i, End: len(lines), SignatureEnd: i + 1})
			active = len(entries) - 1
			firstCode, sigs = false, true
			continue
		}
		if active >= 0 && sigs {
			if match := signatureLine.FindStringSubmatch(trim); match != nil {
				entries[active].Signatures = append(entries[active].Signatures, match[1])
				entries[active].SignatureEnd = i + 1
			} else if trim != "" {
				sigs = false
			}
		}
	}
	return entries
}

func openingFence(line string) string {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	i := 1
	for i < len(line) && line[i] == line[0] {
		i++
	}
	if i < 3 {
		return ""
	}
	return line[:i]
}

func closesFence(line, fence string) bool {
	return strings.HasPrefix(line, fence) && strings.Trim(line, string(fence[0])+" \t") == ""
}

func prose(value string) string { return html.EscapeString(strings.Join(strings.Fields(value), " ")) }

// RenderDraft owns all structural Markdown; model text cannot inject entries,
// signatures or closing command fences. Signatures come from the caller only.
func RenderDraft(d distill.Draft, signature string) (string, error) {
	if !validSignature.MatchString(signature) {
		return "", errors.New("invalid Sage signature")
	}
	if strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.What) == "" || strings.TrimSpace(d.Why) == "" || strings.TrimSpace(d.Example) == "" {
		return "", errors.New("incomplete Sage draft")
	}
	var commands []string
	fence := "```"
	for _, command := range d.Commands {
		command = strings.TrimSpace(strings.ReplaceAll(command, "\r\n", "\n"))
		if command == "" {
			continue
		}
		if strings.ContainsRune(command, '\x00') {
			return "", errors.New("invalid Sage command")
		}
		commands = append(commands, command)
		for strings.Contains(command, fence) {
			fence += "`"
		}
	}
	if len(commands) == 0 {
		return "", errors.New("Sage draft has no commands")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "### %s\n<!-- sig: %s -->\n\n%sbash\n%s\n%s\n", prose(d.Title), signature, fence, strings.Join(commands, "\n"), fence)
	for _, field := range [][2]string{{"What it does", d.What}, {"Why", d.Why}, {"Example", d.Example}, {"Notes", d.Notes}} {
		if value := prose(field[1]); value != "" {
			fmt.Fprintf(&out, "\n- **%s:** %s", field[0], value)
		}
	}
	return out.String() + "\n", nil
}
