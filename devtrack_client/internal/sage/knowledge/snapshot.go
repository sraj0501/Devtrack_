package knowledge

import (
	"errors"
	"strings"
)

// Document is a complete, current Markdown entry. Paths are relative to Root.
type Document struct {
	Filename, Heading, Body string
	Line                    int
	Commands, Signatures    []string
}

// Snapshot reads authoritative Markdown without rewriting user content. Callers
// hold the database writer lock to exclude participating publication/merge processes.
// An interrupted merge must be recovered before exposing either half to search.
func (w Writer) Snapshot() ([]Document, error) {
	publicationMu.Lock()
	defer publicationMu.Unlock()
	if err := w.prepare(); err != nil {
		return nil, err
	}
	journal, err := w.read(".merge.json")
	if err != nil {
		return nil, err
	}
	if len(journal) != 0 {
		return nil, errors.New("Sage merge recovery pending; retry the merge")
	}
	names, err := w.topics()
	if err != nil {
		return nil, err
	}
	var docs []Document
	total := 0
	for _, name := range names {
		raw, err := w.read(name)
		if err != nil {
			return nil, err
		}
		total += len(raw)
		if total > 64<<20 {
			return nil, errors.New("Sage search snapshot exceeds size limit")
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if !balancedFences(text) {
			return nil, errors.New("unclosed Sage topic code fence")
		}
		lines := strings.Split(text, "\n")
		for _, entry := range ParseMarkdown(text) {
			docs = append(docs, Document{Filename: name, Heading: entry.Heading,
				Body: strings.Join(lines[entry.Start:entry.End], "\n"), Line: entry.Start + 1,
				Commands: entry.Commands, Signatures: entry.Signatures})
		}
	}
	return docs, nil
}
