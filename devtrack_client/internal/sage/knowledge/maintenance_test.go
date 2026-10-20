package knowledge

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
)

func maintenanceDraft(needle string) distill.Draft {
	return distill.Draft{Title: "Portable entry", Section: "Operations", Commands: []string{"git status"}, What: needle, Why: "Keep knowledge portable", Example: "Before work"}
}

func TestRepairValidatesMarkdownAndRegeneratesIndex(t *testing.T) {
	root := t.TempDir()
	w := Writer{Root: root}
	if _, err := w.Write("git", "abcdef01", maintenanceDraft("repairneedle")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	result, err := w.Repair()
	if err != nil || result.Topics != 1 || result.Entries != 1 {
		t.Fatalf("repair=%+v err=%v", result, err)
	}
	index, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || !bytes.Contains(index, []byte("| [git](git.md) | 1 |")) {
		t.Fatalf("index=%q err=%v", index, err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.md"), []byte("### Broken\n```sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if _, err := w.Repair(); err == nil {
		t.Fatal("unclosed fence accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed repair changed index")
	}
}

func TestDeleteTopicIsExactRemovesRoutesAndRepairsIndex(t *testing.T) {
	root := t.TempDir()
	w := Writer{Root: root}
	if _, err := w.Write("git", "abcdef01", maintenanceDraft("delete me")); err != nil {
		t.Fatal(err)
	}
	if err := w.RememberRoute("git", "git"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.DeleteTopic("../git"); err == nil {
		t.Fatal("path-like topic accepted")
	}
	removed, err := w.DeleteTopic("git.md")
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "git.md")); !os.IsNotExist(err) {
		t.Fatalf("topic remained: %v", err)
	}
	routes, err := w.Routes()
	if err != nil || len(routes) != 0 {
		t.Fatalf("routes=%+v err=%v", routes, err)
	}
	index, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || bytes.Contains(index, []byte("git.md")) {
		t.Fatalf("stale index=%q err=%v", index, err)
	}
	if _, err := w.DeleteTopic("git"); err == nil {
		t.Fatal("missing topic did not fail")
	}
}

func TestMarkdownExportImportIsDeterministicAdditiveAndIdempotent(t *testing.T) {
	source := Writer{Root: t.TempDir()}
	if _, err := source.Write("git", "abcdef01", maintenanceDraft("exportneedle")); err != nil {
		t.Fatal(err)
	}
	exportRoot := filepath.Join(t.TempDir(), "snapshot")
	exported, err := source.ExportMarkdown(exportRoot)
	if err != nil || exported.Files != 2 || exported.Entries != 1 {
		t.Fatalf("export=%+v err=%v", exported, err)
	}
	if _, err := source.ExportMarkdown(exportRoot); err == nil {
		t.Fatal("existing export destination accepted")
	}
	if _, err := source.ExportMarkdown(filepath.Join(source.Root, "nested")); err == nil {
		t.Fatal("overlapping export accepted")
	}

	target := Writer{Root: t.TempDir()}
	imported, err := target.ImportMarkdown(exportRoot)
	if err != nil || imported.Added != 1 || imported.Unchanged != 0 {
		t.Fatalf("import=%+v err=%v", imported, err)
	}
	first, err := os.ReadFile(filepath.Join(target.Root, "git.md"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := target.ImportMarkdown(exportRoot)
	if err != nil || again.Added != 0 || again.Unchanged != 1 {
		t.Fatalf("repeat=%+v err=%v", again, err)
	}
	second, _ := os.ReadFile(filepath.Join(target.Root, "git.md"))
	if !bytes.Equal(first, second) {
		t.Fatal("repeat import changed topic bytes")
	}
	if _, err := target.ImportMarkdown(target.Root); err == nil {
		t.Fatal("overlapping import accepted")
	}
}

func TestMarkdownImportConflictPreflightDoesNotPartiallyWrite(t *testing.T) {
	sourceRoot := t.TempDir()
	source := Writer{Root: sourceRoot}
	if _, err := source.Write("a", "abcdef01", maintenanceDraft("source-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Write("b", "abcdef02", maintenanceDraft("source-b")); err != nil {
		t.Fatal(err)
	}
	target := Writer{Root: t.TempDir()}
	if _, err := target.Write("a", "abcdef03", maintenanceDraft("different-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ImportMarkdown(sourceRoot); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target.Root, "b.md")); !os.IsNotExist(err) {
		t.Fatalf("preflight wrote b.md: %v", err)
	}
}

func TestMarkdownImportRejectsSymlinkedSource(t *testing.T) {
	realRoot := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := (Writer{Root: t.TempDir()}).ImportMarkdown(link); err == nil {
		t.Fatal("symlinked source accepted")
	}
}

func TestDeleteTopicFaultRollsBackTopicRoutesAndIndex(t *testing.T) {
	root := t.TempDir()
	w := Writer{Root: root}
	if _, err := w.Write("git", "abcdef01", maintenanceDraft("rollback-delete")); err != nil {
		t.Fatal(err)
	}
	if err := w.RememberRoute("git", "git"); err != nil {
		t.Fatal(err)
	}
	topicBefore, _ := os.ReadFile(filepath.Join(root, "git.md"))
	routesBefore, _ := os.ReadFile(filepath.Join(root, routesFile))
	indexBefore, _ := os.ReadFile(filepath.Join(root, "README.md"))
	w.replace = func(path string, raw []byte) error {
		if filepath.Base(path) == "README.md" {
			return errors.New("injected index failure")
		}
		return atomicReplace(path, raw)
	}
	if _, err := w.DeleteTopic("git"); err == nil {
		t.Fatal("injected delete failure succeeded")
	}
	for path, before := range map[string][]byte{
		filepath.Join(root, "git.md"):    topicBefore,
		filepath.Join(root, routesFile):  routesBefore,
		filepath.Join(root, "README.md"): indexBefore,
	} {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("rollback mismatch for %s: %q err=%v", path, after, err)
		}
	}
}

func TestMarkdownImportFaultRollsBackAllAddedFiles(t *testing.T) {
	source := Writer{Root: t.TempDir()}
	if _, err := source.Write("a", "abcdef01", maintenanceDraft("source-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Write("b", "abcdef02", maintenanceDraft("source-b")); err != nil {
		t.Fatal(err)
	}
	targetRoot := t.TempDir()
	target := Writer{Root: targetRoot}
	target.replace = func(path string, raw []byte) error {
		if filepath.Base(path) == "README.md" {
			return errors.New("injected index failure")
		}
		return atomicReplace(path, raw)
	}
	if _, err := target.ImportMarkdown(source.Root); err == nil {
		t.Fatal("injected import failure succeeded")
	}
	for _, name := range []string{"a.md", "b.md", "README.md"} {
		if _, err := os.Stat(filepath.Join(targetRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("partial import file remained: %s err=%v", name, err)
		}
	}
}
