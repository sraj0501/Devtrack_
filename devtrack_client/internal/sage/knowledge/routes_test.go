package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func routeDocument(t *testing.T, w Writer, name, title, command string) {
	t.Helper()
	d := sampleDraft()
	d.Commands = []string{command}
	body, err := RenderDraft(d, "aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, name), []byte("# "+title+"\n\n## Group\n\n"+body), 0600); err != nil {
		t.Fatal(err)
	}
}

func routeFor(t *testing.T, w Writer, binary string) Route {
	t.Helper()
	routes, err := w.Routes()
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		if route.Binary == binary {
			return route
		}
	}
	return Route{}
}

func TestReferenceAnExistingEntryRoutesItsWholeBinary(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "git.md", "Git", "git status --short")
	if got := routeFor(t, w, "git"); got.Filename != "git.md" || got.Title != "Git" || got.Source != "entries" {
		t.Fatal(got)
	}
	d := sampleDraft()
	d.Commands = []string{"git log"}
	if name, err := w.Write("git", "bbbbbbbbbbbb", d); err != nil || name != "git.md" {
		t.Fatalf("%s %v", name, err)
	}
}

func TestReferenceABinaryTheListNeverKnewStillRoutes(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "package-managers.md", "Package managers", "dnf install -y ripgrep")
	if got := routeFor(t, w, "dnf"); got.Filename != "package-managers.md" {
		t.Fatal(got)
	}
}

func TestReferenceUnknownBinaryHasNoRouteYet(t *testing.T) {
	if got := routeFor(t, Writer{Root: t.TempDir()}, "nixos-rebuild"); got != (Route{}) {
		t.Fatal(got)
	}
}

func TestReferenceRememberedDecisionsSurviveBeforeAnyEntryExists(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if err := w.RememberRoute("nixos-rebuild", "nix.md"); err != nil {
		t.Fatal(err)
	}
	if got := routeFor(t, Writer{Root: w.Root}, "nixos-rebuild"); got.Filename != "nix.md" || got.Source != "remembered" {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "nix.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("route prematurely wrote topic: %v", err)
	}
}

func TestReferenceEntriesWinOverAStaleRememberedRoute(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if err := w.RememberRoute("git", "wrong.md"); err != nil {
		t.Fatal(err)
	}
	routeDocument(t, w, "git.md", "Git", "git status --short")
	if got := routeFor(t, w, "git"); got.Filename != "git.md" {
		t.Fatal(got)
	}
	if err := os.Rename(filepath.Join(w.Root, "git.md"), filepath.Join(w.Root, "version-control.md")); err != nil {
		t.Fatal(err)
	}
	if got := routeFor(t, Writer{Root: w.Root}, "git"); got.Filename != "version-control.md" {
		t.Fatal(got)
	}
	if name, err := w.Write("git", "bbbbbbbbbbbb", sampleDraft()); err != nil || name != "version-control.md" {
		t.Fatalf("%s %v", name, err)
	}
}

func TestReferenceTitleIsReadBackFromTheFile(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "kubernetes.md", "Kubernetes", "kubectl get pods")
	if got := routeFor(t, w, "kubectl"); got.Title != "Kubernetes" {
		t.Fatal(got)
	}
}

func TestReferenceSkippedAndReadmeNeverBecomeRoutes(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	for _, name := range []string{"_skipped.md", "README.md"} {
		routeDocument(t, w, name, "Skipped", "date")
	}
	if got := routeFor(t, w, "date"); got != (Route{}) {
		t.Fatal(got)
	}
}

func TestReferenceFallbackIsAFileOfItsOwn(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	d := sampleDraft()
	d.Commands = []string{"cargo test"}
	if name, err := w.Write("cargo", "aaaaaaaaaaaa", d); err != nil || name != "cargo.md" {
		t.Fatalf("%s %v", name, err)
	}
}

func TestReferenceACorrectionOutlivesTheClassifier(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if err := w.RememberRoute("dnf", "package-managers.md"); err != nil {
		t.Fatal(err)
	}
	d := sampleDraft()
	d.Commands = []string{"dnf install ripgrep"}
	if name, err := (Writer{Root: w.Root}).Write("dnf", "aaaaaaaaaaaa", d); err != nil || name != "package-managers.md" {
		t.Fatalf("%s %v", name, err)
	}
}

func TestSageRoutesValidatePreserveAndAtomicallyReplace(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	for _, binary := range []string{"../bad", "bad name", "$(secret)", ""} {
		if err := w.RememberRoute(binary, "x"); err == nil {
			t.Fatalf("accepted %q", binary)
		}
	}
	if err := w.RememberRoute("Git", "../../version control.md"); err != nil {
		t.Fatal(err)
	}
	before := readTopic(t, w.Root, routesFile)
	w.replace = func(string, []byte) error { return errors.New("injected replacement failure") }
	if err := w.RememberRoute("go", "golang"); err == nil {
		t.Fatal("expected error")
	}
	if after := readTopic(t, w.Root, routesFile); after != before {
		t.Fatal("failed write lost routes")
	}
	w.replace = nil
	if err := w.RememberRoute("go", "golang"); err != nil {
		t.Fatal(err)
	}
	routes, err := w.Routes()
	if err != nil || len(routes) != 2 || routes[0].Binary != "git" || routes[1].Binary != "go" {
		t.Fatalf("%+v %v", routes, err)
	}
	if routes[0].Filename != SafeFilename("../../version control.md") {
		t.Fatal(routes)
	}
	for _, raw := range []string{`{"git":"../../outside.md"}`, `{"git":42}`, `null`, `{CANARY`} {
		if err := os.WriteFile(filepath.Join(w.Root, routesFile), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Routes(); err == nil || strings.Contains(err.Error(), "CANARY") {
			t.Fatalf("unsafe diagnostic: %v", err)
		}
		if err := w.RememberRoute("go", "go"); err == nil {
			t.Fatal("overwrote malformed routes")
		}
		if got := readTopic(t, w.Root, routesFile); got != raw {
			t.Fatal("lost invalid file")
		}
	}
}

func TestSageRoutePrecedenceIsStableAndFenceAware(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	routeDocument(t, w, "z.md", "Z", "git status")
	routeDocument(t, w, "a.md", "A", "git diff")
	want := Route{"git", "a.md", "A", "entries"}
	if got := routeFor(t, w, "git"); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if got := topicTitle("a.md", "```\n# Fake\n```\n# Real\n"); got != "Real" {
		t.Fatal(got)
	}
}
