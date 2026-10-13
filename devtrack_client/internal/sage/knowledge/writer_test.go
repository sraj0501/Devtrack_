package knowledge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

func sampleDraft() distill.Draft {
	return distill.Draft{Title: "Check what changed before committing", Section: "Committing", Commands: []string{"git --no-pager diff --staged"}, What: "Shows staged changes.", Why: "Review before committing.", Example: "git diff --staged"}
}

func referenceResponse(notes string) string {
	return `{"worth_documenting":true,"title":"Inspect repository state","section":"Inspecting",` +
		`"commands":["git status --short"],"what":"Shows changes","why":"Review work",` +
		`"example":"Before committing","notes":` + fmt.Sprintf("%q", notes) + `}`
}

func TestReferenceCleanResponse(t *testing.T) {
	outcome, err := distill.Parse(referenceResponse("Useful note"), "git status")
	if err != nil || outcome.Draft == nil {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	raw, err := RenderDraft(*outcome.Draft, "abcdef01")
	if err != nil {
		t.Fatal(err)
	}
	entries := ParseMarkdown(raw)
	if len(entries) != 1 || entries[0].Heading != "Inspect repository state" ||
		len(entries[0].Commands) != 1 || entries[0].Commands[0] != "git status --short" {
		t.Fatalf("entry=%+v raw=%s", entries, raw)
	}
}

func TestReferenceAnswerWrappedInAFenceOrChatter(t *testing.T) {
	raw := "Here is the entry:\n```json\n" + referenceResponse("") + "\n```\nDone"
	outcome, err := distill.Parse(raw, "git status")
	if err != nil || outcome.Draft == nil {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if _, err := RenderDraft(*outcome.Draft, "abcdef02"); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceModelEmittedSigIsStripped(t *testing.T) {
	raw := strings.Replace(referenceResponse(""), "Inspect repository state", "Inspect <!-- sig: deadbeef --> state", 1)
	outcome, err := distill.Parse(raw, "git status")
	if err != nil || strings.Contains(outcome.Draft.Title, "sig:") {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	rendered, err := RenderDraft(*outcome.Draft, "abcdef03")
	if err != nil || strings.Count(rendered, "<!-- sig:") != 1 || !strings.Contains(rendered, "abcdef03") {
		t.Fatalf("rendered=%q err=%v", rendered, err)
	}
}

func TestReferenceGarbageIsRejectedNotWritten(t *testing.T) {
	root := t.TempDir()
	if _, err := distill.Parse("not JSON", "git status"); err == nil {
		t.Fatal("garbage response accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("garbage wrote files: %v err=%v", entries, err)
	}
}

func TestReferenceRendersAParseableEntry(t *testing.T) {
	raw, err := RenderDraft(sampleDraft(), "abcdef04")
	entries := ParseMarkdown(raw)
	if err != nil || len(entries) != 1 || entries[0].Signatures[0] != "abcdef04" || len(entries[0].Commands) != 1 {
		t.Fatalf("entries=%+v raw=%q err=%v", entries, raw, err)
	}
}

func TestReferenceBlankNotesOmitsTheBullet(t *testing.T) {
	draft := sampleDraft()
	draft.Notes = " \n "
	raw, err := RenderDraft(draft, "abcdef05")
	if err != nil || strings.Contains(raw, "**Notes:**") {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}

func TestReferenceMultilineProseIsFlattened(t *testing.T) {
	draft := sampleDraft()
	draft.What = "Shows\n  staged\tchanges"
	raw, err := RenderDraft(draft, "abcdef06")
	if err != nil || !strings.Contains(raw, "Shows staged changes") || strings.Contains(raw, "Shows\n") {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}

func TestReferenceDocumentedSigsSpansEveryFile(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if _, err := w.Write("git", "abcdef11", sampleDraft()); err != nil {
		t.Fatal(err)
	}
	draft := sampleDraft()
	draft.Commands = []string{"docker ps"}
	if _, err := w.Write("docker", "abcdef12", draft); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteSkipped("abcdef13"); err != nil {
		t.Fatal(err)
	}
	signatures, err := w.DocumentedSignatures()
	if err != nil || len(signatures) != 3 || signatures["abcdef11"] != "git.md" ||
		signatures["abcdef12"] != "docker.md" || signatures["abcdef13"] != "_skipped.md" {
		t.Fatalf("signatures=%+v err=%v", signatures, err)
	}
}

func TestReferenceExactAndCaseInsensitiveMatchReuse(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "abcdef31", "Git Workflows")
	writeDraft(t, w, "abcdef32", "git workflows")
	raw := readTopic(t, w.Root, "git.md")
	if strings.Count(raw, "\n## ") != 1 || len(ParseMarkdown(raw)) != 2 {
		t.Fatalf("section was duplicated: %s", raw)
	}
}

func TestReferenceNearDuplicateSnapsToExisting(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "abcdef33", "Docker Containers")
	writeDraft(t, w, "abcdef34", "Docker Container")
	raw := readTopic(t, w.Root, "git.md")
	if strings.Count(raw, "\n## ") != 1 || !strings.Contains(raw, "## Docker Containers") {
		t.Fatalf("near duplicate did not snap: %s", raw)
	}
}

func TestReferenceGenuinelyNewSectionIsKept(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "abcdef35", "Testing")
	writeDraft(t, w, "abcdef36", "Deployment")
	raw := readTopic(t, w.Root, "git.md")
	if strings.Count(raw, "\n## ") != 2 {
		t.Fatalf("new section was merged: %s", raw)
	}
}

func TestReferenceRelatedButDifferentlyWordedSectionsAreNotMerged(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "abcdef37", "Git Workflows")
	writeDraft(t, w, "abcdef38", "Version Control")
	raw := readTopic(t, w.Root, "git.md")
	if strings.Count(raw, "\n## ") != 2 {
		t.Fatalf("different wording was merged: %s", raw)
	}
}

func TestReferenceEmptyFallsBackToDefault(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "abcdef39", "")
	raw := readTopic(t, w.Root, "git.md")
	if !strings.Contains(raw, "## Other") {
		t.Fatalf("missing default section: %s", raw)
	}
}

func readTopic(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func writeDraft(t *testing.T, w Writer, sig, section string) string {
	t.Helper()
	d := sampleDraft()
	d.Section = section
	name, err := w.Write("Git", sig, d)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestReferenceFilenameCannotEscapeTheKnowledgeBase(t *testing.T) {
	for _, hostile := range []string{"../../etc/passwd", "/etc/shadow", "a/b/c", "..", "....//....//x.md", `C:\Windows\x`, "a:stream", "CON", "nul.txt", "LPT1", "README.md", "_skipped.md"} {
		name := SafeFilename(hostile)
		if strings.ContainsAny(name, `/\:`) || filepath.Base(name) != name || windowsDevice.MatchString(name) || strings.EqualFold(name, "README.md") || strings.HasPrefix(name, "_") {
			t.Fatalf("unsafe %q -> %q", hostile, name)
		}
	}
	for input, want := range map[string]string{"": "misc.md", "Shell.md": "shell.md", "package managers": "package-managers.md"} {
		if got := SafeFilename(input); got != want {
			t.Fatalf("%q: %q", input, got)
		}
	}
}

func TestReferenceParsesHeadingSigsAndCommands(t *testing.T) {
	body, err := RenderDraft(sampleDraft(), "aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	entries := ParseMarkdown("## Committing\n\n" + body)
	if len(entries) != 1 {
		t.Fatal(entries)
	}
	e := entries[0]
	if e.Heading != sampleDraft().Title || e.Section != "Committing" || !reflect.DeepEqual(e.Signatures, []string{"aaaaaaaaaaaa"}) || !reflect.DeepEqual(e.Commands, sampleDraft().Commands) {
		t.Fatalf("%+v", e)
	}
}

func TestReferenceMultipleSigsOnOneEntry(t *testing.T) {
	text := "### Entry\n<!-- sig: aaaaaaaaaaaa -->\n<!-- sig: bbbbbbbbbbbb -->\n\n```bash\ngit diff\n```\n"
	if got := ParseMarkdown(text); len(got) != 1 || !reflect.DeepEqual(got[0].Signatures, []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb"}) {
		t.Fatal(got)
	}
}

func TestReferenceAddSigAppendsAndIsIdempotent(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	name := writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
	changed, err := w.AddSignature("Git", "aaaaaaaaaaaa", "cccccccccccc")
	if err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	before := readTopic(t, w.Root, name)
	changed, err = w.AddSignature("Git", "aaaaaaaaaaaa", "cccccccccccc")
	if err != nil || changed || before != readTopic(t, w.Root, name) {
		t.Fatalf("%v %v", changed, err)
	}
	e := ParseMarkdown(before)
	if len(e) != 1 || !reflect.DeepEqual(e[0].Signatures, []string{"aaaaaaaaaaaa", "cccccccccccc"}) || !strings.Contains(before, sampleDraft().Commands[0]) {
		t.Fatal(e)
	}
}

func TestReferenceInsertCreatesFileWithHeader(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	name := writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
	text := readTopic(t, w.Root, name)
	if !strings.HasPrefix(text, "# Git\n") || !strings.Contains(text, "## Committing\n") || len(ParseMarkdown(text)) != 1 {
		t.Fatal(text)
	}
}

func TestReferenceInsertReusesAnExistingSection(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
	name := writeDraft(t, w, "bbbbbbbbbbbb", "committing")
	text := readTopic(t, w.Root, name)
	if strings.Count(text, "## Committing\n") != 1 || strings.Contains(text, "## committing\n") || len(ParseMarkdown(text)) != 2 {
		t.Fatal(text)
	}
}

func TestReferenceInsertPreservesEarlierEntries(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	name := writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
	before := readTopic(t, w.Root, name)
	writeDraft(t, w, "eeeeeeeeeeee", "Branching")
	text := readTopic(t, w.Root, name)
	if !strings.HasPrefix(text, before) || !strings.Contains(text, "## Branching") {
		t.Fatal(text)
	}
	writeDraft(t, w, "dddddddddddd", "Committing")
	entries := ParseMarkdown(readTopic(t, w.Root, name))
	if len(entries) != 3 || entries[0].Signatures[0] != "aaaaaaaaaaaa" || entries[1].Signatures[0] != "dddddddddddd" || entries[2].Section != "Branching" {
		t.Fatal(entries)
	}
}

func TestMarkdownWriterReplayPreservesCorrectedLocationAndIndex(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	name := writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
	before := readTopic(t, w.Root, name)
	if err := os.Rename(filepath.Join(w.Root, name), filepath.Join(w.Root, "version-control.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "README.md"), []byte("# My notes\n\nKeep this introduction.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := w.Write("wrong-topic", "aaaaaaaaaaaa", sampleDraft())
		if err != nil || got != "version-control.md" {
			t.Fatalf("%q %v", got, err)
		}
	}
	if before != readTopic(t, w.Root, "version-control.md") {
		t.Fatal("replay modified existing prose")
	}
	index := readTopic(t, w.Root, "README.md")
	if !strings.Contains(index, "Keep this introduction.") || strings.Count(index, "(version-control.md)") != 1 {
		t.Fatal(index)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "wrong-topic.md")); !os.IsNotExist(err) {
		t.Fatal("replay created another topic")
	}
}

func TestMarkdownWriterFailureRecovery(t *testing.T) {
	for _, failedFile := range []string{"git.md", "README.md"} {
		t.Run(failedFile, func(t *testing.T) {
			w := Writer{Root: t.TempDir()}
			writeDraft(t, w, "aaaaaaaaaaaa", "Committing")
			old := readTopic(t, w.Root, "git.md")
			w.replace = func(path string, raw []byte) error {
				if filepath.Base(path) == failedFile {
					return errors.New("injected replacement failure")
				}
				return atomicReplace(path, raw)
			}
			if _, err := w.Write("Git", "bbbbbbbbbbbb", sampleDraft()); err == nil {
				t.Fatal("acknowledged failed write")
			}
			if failedFile == "git.md" && old != readTopic(t, w.Root, "git.md") {
				t.Fatal("failed replacement damaged original")
			}
			w.replace = nil
			writeDraft(t, w, "bbbbbbbbbbbb", "Committing")
			before := readTopic(t, w.Root, "git.md")
			index := readTopic(t, w.Root, "README.md")
			writeDraft(t, w, "bbbbbbbbbbbb", "Committing")
			if len(ParseMarkdown(before)) != 2 || before != readTopic(t, w.Root, "git.md") || index != readTopic(t, w.Root, "README.md") || !strings.Contains(index, "| 2 |") {
				t.Fatal("recovery was not byte-stable")
			}
		})
	}
}

func TestMarkdownWriterRejectsLinksAndMalformedIndex(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(w.Root, "README.md"), []byte("private introduction\n"+indexStart), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write("git", "aaaaaaaaaaaa", sampleDraft()); err == nil {
		t.Fatal("accepted broken index")
	}
	if got := readTopic(t, w.Root, "README.md"); got != "private introduction\n"+indexStart {
		t.Fatal("overwrote index")
	}
	out := t.TempDir()
	path := filepath.Join(out, "outside.md")
	if err := os.WriteFile(path, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(w.Root, "linked.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := w.Write("linked", "bbbbbbbbbbbb", sampleDraft()); err == nil {
		t.Fatal("followed symlink")
	}
	if readTopic(t, out, "outside.md") != "outside" {
		t.Fatal("modified link target")
	}
}

func TestMarkdownWriterConcurrentUpdates(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := (Writer{Root: root}).Write("git", fmt.Sprintf("%012x", i), sampleDraft())
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if got := len(ParseMarkdown(readTopic(t, root, "git.md"))); got != 20 {
		t.Fatal(got)
	}
}

func TestMarkdownWriterPreservesMalformedTopic(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	text := "# Git\n\n```bash\nunfinished user edit\n"
	if err := os.WriteFile(filepath.Join(w.Root, "git.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write("git", "aaaaaaaaaaaa", sampleDraft()); err == nil {
		t.Fatal("appended inside unclosed code fence")
	}
	if readTopic(t, w.Root, "git.md") != text {
		t.Fatal("modified malformed topic")
	}
}

func TestRenderDraftGoldenAndStructuralInjection(t *testing.T) {
	got, err := RenderDraft(sampleDraft(), "aaaaaaaaaaaa")
	want := "### Check what changed before committing\n<!-- sig: aaaaaaaaaaaa -->\n\n```bash\ngit --no-pager diff --staged\n```\n\n- **What it does:** Shows staged changes.\n- **Why:** Review before committing.\n- **Example:** git diff --staged\n"
	if err != nil || got != want {
		t.Fatalf("%v\n%s", err, got)
	}
	d := sampleDraft()
	d.Title = "title\n### forged\n<!-- sig: bbbbbbbbbbbb -->"
	d.Commands = []string{"echo hi\n```\n### fake\n<!-- sig: cccccccccccc -->\n~~~\n## fake section"}
	body, err := RenderDraft(d, "aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	entries := ParseMarkdown(body)
	if len(entries) != 1 || !reflect.DeepEqual(entries[0].Signatures, []string{"aaaaaaaaaaaa"}) || len(entries[0].Commands) != 6 {
		t.Fatalf("%+v\n%s", entries, body)
	}
}

func FuzzMarkdownRoundTrip(f *testing.F) {
	for _, seed := range []string{"git status", "```\n### forged", "~~~\n<!-- sig: aaaaaa -->", "\r\n## fake"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, command string) {
		d := sampleDraft()
		d.Commands = []string{command}
		body, err := RenderDraft(d, "abcdef123456")
		if err != nil {
			return
		}
		entries := ParseMarkdown(body)
		if len(entries) != 1 || !reflect.DeepEqual(entries[0].Signatures, []string{"abcdef123456"}) {
			t.Fatalf("structure changed: %+v", entries)
		}
	})
}
