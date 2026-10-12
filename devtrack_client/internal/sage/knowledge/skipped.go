package knowledge

import (
	"errors"
	"strings"
)

const skippedFile = "_skipped.md"

// WriteSkipped records a validated skip without retaining model prose or raw
// command arguments. The caller supplies the stable, hashed action signature.
// As with Write, cross-process callers must hold the shared database writer lock.
func (w Writer) WriteSkipped(signature string) (string, error) {
	if !validSignature.MatchString(signature) {
		return "", errors.New("invalid Sage signature")
	}
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return "", err
	}
	if _, err := w.recoverMerge(); err != nil {
		return "", err
	}
	raw, err := w.read(skippedFile)
	if err != nil {
		return "", err
	}
	if !balancedFences(string(raw)) {
		return "", errors.New("unclosed Sage skipped code fence")
	}
	for _, entry := range ParseMarkdown(string(raw)) {
		for _, sig := range entry.Signatures {
			if sig == signature {
				return skippedFile, nil
			}
		}
	}
	text := string(raw)
	if text == "" {
		text = "# Skipped\n\n> Actions explicitly judged not useful as reusable knowledge.\n"
	}
	text = strings.TrimRight(text, "\n") + "\n\n### Skipped action\n<!-- sig: " + signature + " -->\n\nModel judged command not useful.\n"
	return skippedFile, w.write(skippedFile, []byte(text))
}
