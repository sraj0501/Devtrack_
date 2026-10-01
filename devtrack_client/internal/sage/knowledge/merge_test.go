package knowledge

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceMergeMovesEntriesAndRepointsRouting(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "shell.md", "Shell", "rg -n foo")
	routeDocument(t, w, "grep.md", "Grep", "grep -rn foo .")
	raw, _ := w.read("grep.md")
	raw = bytes.ReplaceAll(raw, []byte("aaaaaaaaaaaa"), []byte("bbbbbbbbbbbb"))
	if err := w.write("grep.md", raw); err != nil {
		t.Fatal(err)
	}
	if err := w.RememberRoute("find", "grep"); err != nil {
		t.Fatal(err)
	}
	if n, err := w.Merge("grep.md", "shell.md"); n != 1 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
	got, err := w.read("shell.md")
	if err != nil || len(ParseMarkdown(string(got))) != 2 || !bytes.Contains(got, []byte("aaaaaaaaaaaa")) || !bytes.Contains(got, []byte("bbbbbbbbbbbb")) {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "grep.md")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, binary := range []string{"grep", "find"} {
		if routeFor(t, w, binary).Filename != "shell.md" {
			t.Fatal(binary)
		}
	}
	index, _ := w.read("README.md")
	if bytes.Contains(index, []byte("grep.md")) || !bytes.Contains(index, []byte("shell.md")) {
		t.Fatal(string(index))
	}
	if n, err := w.Merge("grep", "shell"); n != 0 || err != nil {
		t.Fatalf("repeat: %d %v", n, err)
	}
	after, _ := w.read("shell.md")
	if !bytes.Equal(got, after) {
		t.Fatal("repeat changed destination")
	}
}

func TestReferenceMergeIsANoOpForAMissingFile(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	for _, pair := range [][2]string{{"missing", "shell"}, {"shell", "shell"}} {
		if n, err := w.Merge(pair[0], pair[1]); n != 0 || err != nil {
			t.Fatalf("%d %v", n, err)
		}
	}
	files, _ := os.ReadDir(w.Root)
	if len(files) != 0 {
		t.Fatal(files)
	}
}

func TestSageMergeRecoveryAtEveryWriteBoundary(t *testing.T) {
	for _, fail := range []string{mergeFile, "shell.md", routesFile, "README.md"} {
		t.Run(fail, func(t *testing.T) {
			w := Writer{Root: t.TempDir()}
			routeDocument(t, w, "grep.md", "Grep", "grep foo")
			original, _ := w.read("grep.md")
			w.replace = func(path string, raw []byte) error {
				if filepath.Base(path) == fail {
					return errors.New("injected write failure")
				}
				return atomicReplace(path, raw)
			}
			if _, err := w.Merge("grep", "shell"); err == nil {
				t.Fatal("expected failure")
			}
			fresh := Writer{Root: w.Root}
			if n, err := fresh.Merge("grep", "shell"); err != nil || n != 1 {
				t.Fatalf("recover: %d %v", n, err)
			}
			raw, _ := fresh.read("shell.md")
			if len(ParseMarkdown(string(raw))) != 1 || !bytes.Contains(raw, []byte("aaaaaaaaaaaa")) {
				t.Fatal(string(raw))
			}
			entries := ParseMarkdown(string(original))
			body := strings.TrimSpace(strings.Join(strings.Split(string(original), "\n")[entries[0].Start:entries[0].End], "\n"))
			if !strings.Contains(string(raw), body) {
				t.Fatal("entry prose changed")
			}
			if _, err := os.Stat(filepath.Join(w.Root, mergeFile)); !os.IsNotExist(err) {
				t.Fatal("journal remains", err)
			}
		})
	}
}

func TestSageMergeRecoveryPreservesLaterEdits(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "grep.md", "Grep", "grep foo")
	w.replace = func(path string, raw []byte) error {
		if filepath.Base(path) == routesFile {
			return errors.New("interrupted")
		}
		return atomicReplace(path, raw)
	}
	if _, err := w.Merge("grep", "shell"); err == nil {
		t.Fatal("expected failure")
	}
	fresh := Writer{Root: w.Root}
	edited := []byte("# Shell\n\nUser edits after interruption\n")
	if err := os.WriteFile(filepath.Join(w.Root, "shell.md"), edited, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := fresh.Merge("grep", "shell"); err == nil {
		t.Fatal("overwrote edits")
	}
	got, _ := fresh.read("shell.md")
	if !bytes.Equal(got, edited) {
		t.Fatal("lost edits")
	}
	if src, _ := fresh.read("grep.md"); len(src) == 0 {
		t.Fatal("lost source")
	}
	if _, err := fresh.Routes(); err == nil {
		t.Fatal("reported incomplete routes")
	}
}

func TestSageMergeRejectsUnstructuredNotesAndMalformedJournal(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "grep.md", "Grep", "grep foo")
	raw, _ := w.read("grep.md")
	raw = append([]byte("User notes\n"), raw...)
	if err := w.write("grep.md", raw); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Merge("grep", "shell"); err == nil {
		t.Fatal("unstructured notes lost")
	}
	got, _ := w.read("grep.md")
	if !bytes.Equal(raw, got) {
		t.Fatal("source changed")
	}
	if err := w.write(mergeFile, []byte(`{"Source":"../outside.md"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Merge("grep", "shell"); err == nil {
		t.Fatal("unsafe journal accepted")
	}
}

func TestSagePublicationRecoversInterruptedMerge(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "grep.md", "Grep", "grep foo")
	w.replace = func(path string, raw []byte) error {
		if filepath.Base(path) == "README.md" {
			return errors.New("interrupted index")
		}
		return atomicReplace(path, raw)
	}
	if _, err := w.Merge("grep", "shell"); err == nil {
		t.Fatal("expected failure")
	}
	fresh := Writer{Root: w.Root}
	draft := sampleDraft()
	draft.Commands = []string{"grep bar"}
	if name, err := fresh.Write("grep", "bbbbbbbbbbbb", draft); err != nil || name != "shell.md" {
		t.Fatalf("%s %v", name, err)
	}
	raw, _ := fresh.read("shell.md")
	if len(ParseMarkdown(string(raw))) != 2 {
		t.Fatal(string(raw))
	}
}
