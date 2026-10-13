package knowledge

import "strings"

// Action identifies the stable command operation while ignoring later flags
// and values. It intentionally distinguishes real subcommands: git status and
// git log are separate actions, while their flag variants match.
func Action(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 || !ValidRouteBinary(strings.ToLower(fields[0])) {
		return ""
	}
	binary := strings.ToLower(fields[0])
	for i := 1; i < len(fields); i++ {
		field := strings.ToLower(fields[i])
		if strings.HasPrefix(field, "-") {
			continue
		}
		if field == "|" || field == "&&" || field == ";" {
			break
		}
		return binary + " " + field
	}
	return binary
}

func entryAction(entry MarkdownEntry) string {
	for _, command := range entry.Commands {
		if action := Action(command); action != "" {
			return action
		}
	}
	return ""
}

// ResolveExisting returns a deterministic documented destination and an
// existing signature when either the exact signature or command action is
// already present. It never rewrites files.
func (w Writer) ResolveExisting(command, signature string) (filename, existing string, found bool, err error) {
	docs, err := w.Snapshot()
	if err != nil {
		return "", "", false, err
	}
	action := Action(command)
	for _, doc := range docs {
		for _, sig := range doc.Signatures {
			if sig == signature {
				return doc.Filename, sig, true, nil
			}
		}
	}
	if action == "" {
		return "", "", false, nil
	}
	for _, doc := range docs {
		for _, existingCommand := range doc.Commands {
			if Action(existingCommand) == action && len(doc.Signatures) > 0 {
				return doc.Filename, doc.Signatures[0], true, nil
			}
		}
	}
	return "", "", false, nil
}

// DocumentedSignatures spans every topic plus the terminal skipped record.
func (w Writer) DocumentedSignatures() (map[string]string, error) {
	docs, err := w.Snapshot()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, doc := range docs {
		for _, signature := range doc.Signatures {
			out[signature] = doc.Filename
		}
	}
	raw, err := w.read(skippedFile)
	if err != nil {
		return nil, err
	}
	for _, entry := range ParseMarkdown(string(raw)) {
		for _, signature := range entry.Signatures {
			out[signature] = skippedFile
		}
	}
	return out, nil
}
