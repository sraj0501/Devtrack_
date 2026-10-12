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
