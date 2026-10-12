package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func TestReferenceKeywordSearchMatchesWholeEntriesAndFiltersTopics(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w := knowledge.Writer{Root: t.TempDir()}
	draft := distill.Draft{Title: "Review staged work", Section: "Review", Commands: []string{"git diff --staged"}, What: "Inspect the patch", Why: "Catch regression needles", Example: "Before committing", Notes: "Straße notes"}
	if _, err := w.Write("git", "abcdef01", draft); err != nil {
		t.Fatal(err)
	}
	// Other entries must not combine to satisfy AND terms across entry boundaries.
	draft.Title = "Unrelated"
	draft.Commands = []string{"git log"}
	draft.What = "missingword"
	draft.Why = "History"
	draft.Notes = ""
	if _, err := w.Write("git", "abcdef02", draft); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, "git.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"_skipped.md", "README.md"} {
		if err := os.WriteFile(filepath.Join(w.Root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		query, topic string
		count        int
	}{
		{"git staged", "", 1}, {"patch regression", "", 1}, {"staged missingword", "", 0}, {"git staged", "docker", 0},
		{"git staged", "GI", 1}, {"--staged", "git.md", 1}, {"STRASSE", "", 1}, {"git %", "", 0}, {"\"git\"", "", 0},
	} {
		got, err := d.SearchSageEntries(context.Background(), w, tc.query, tc.topic, 20)
		if err != nil || len(got) != tc.count {
			t.Fatalf("%q %q: %d %v", tc.query, tc.topic, len(got), err)
		}
		if len(got) > 0 && (got[0].Filename != "git.md" || !strings.Contains(got[0].Body, "Catch regression needles") || got[0].Line < 1) {
			t.Fatalf("incomplete entry: %+v", got[0])
		}
	}
	topics, err := d.ListSageEntryTopics(context.Background(), w)
	if err != nil || len(topics) != 1 || topics[0].Name != "git" || topics[0].Entries != 2 {
		t.Fatalf("topics: %v %v", topics, err)
	}
}

func TestSageEntrySearchReconcilesMovesEditsDeletesAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sage.db")
	d, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.Close() }()
	w := knowledge.Writer{Root: t.TempDir()}
	ctx := context.Background()
	queueEvent(t, d, "search", "git status")
	q := &SageQueue{Database: d}
	job, _, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	draft := distill.Draft{Title: "Status", Section: "Work", Commands: []string{"git status"}, What: "originalneedle", Why: "Review", Example: "Before work"}
	if err := q.Complete(ctx, job.ID, draft); err != nil {
		t.Fatal(err)
	}
	if _, err := q.PublishNext(ctx, w); err != nil {
		t.Fatal(err)
	}
	check := func(query, topic string, count int, filename string) {
		t.Helper()
		got, err := d.SearchSageEntries(ctx, w, query, topic, 20)
		if err != nil || len(got) != count {
			t.Fatalf("%s: %v %v", query, got, err)
		}
		var actual string
		if err := d.DB().QueryRow(`SELECT filename FROM sage_publications`).Scan(&actual); err != nil || actual != filename {
			t.Fatalf("filename %q expected %q: %v", actual, filename, err)
		}
	}
	check("originalneedle", "git", 1, "git.md")
	if _, err := d.MergeSageTopics(ctx, w, "git", "version-control"); err != nil {
		t.Fatal(err)
	}
	check("originalneedle", "version", 1, "version-control.md")
	check("originalneedle", "git", 0, "version-control.md")
	if err := os.Rename(filepath.Join(w.Root, "version-control.md"), filepath.Join(w.Root, "manual.md")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(w.Root, "manual.md")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(raw), "originalneedle", "editedneedle")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	check("editedneedle", "manual", 1, "manual.md")
	check("originalneedle", "", 0, "manual.md")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	check("editedneedle", "", 0, "")
}

func TestSageEntrySearchRankingAndFailedSnapshot(t *testing.T) {
	d, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w := knowledge.Writer{Root: t.TempDir()}
	ctx := context.Background()
	for _, tc := range []struct{ name, title, cmd string }{{"a", "Other", "git status"}, {"b", "Other", "git needle"}, {"c", "Needle", "git status"}} {
		draft := distill.Draft{Title: tc.title, Section: "Test", Commands: []string{tc.cmd}, What: "needle", Why: "Review", Example: "Example"}
		if _, err := w.Write(tc.name, "abcdef0"+tc.name, draft); err != nil {
			t.Fatal(err)
		}
	}
	got, err := d.SearchSageEntries(ctx, w, "needle", "", 2)
	if err != nil || len(got) != 2 || got[0].Filename != "c.md" || got[1].Filename != "b.md" {
		t.Fatalf("rank: %v %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, ".merge.json"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := d.SearchSageEntries(ctx, w, "needle", "", 20); err == nil || len(got) != 0 {
		t.Fatal("exposed partial merge")
	}
	if err := os.Remove(filepath.Join(w.Root, ".merge.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "z.md"), []byte("### Broken\n```sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := d.SearchSageEntries(ctx, w, "needle", "", 20); err == nil || len(got) != 0 {
		t.Fatal("returned stale results")
	}
	var count int
	if err := d.DB().QueryRow(`SELECT count(*) FROM sage_entries`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("failed scan changed cache: %d %v", count, err)
	}
}
