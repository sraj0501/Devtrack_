package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceSkippedFileRecordsTheSig(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if _, err := w.WriteSkipped("abcdef01"); err != nil {
		t.Fatal(err)
	}
	before := readTopic(t, w.Root, skippedFile)
	if !strings.Contains(before, "<!-- sig: abcdef01 -->") {
		t.Fatal(before)
	}
	if _, err := w.WriteSkipped("abcdef01"); err != nil {
		t.Fatal(err)
	}
	if after := readTopic(t, w.Root, skippedFile); after != before {
		t.Fatal("replay changed skip record")
	}
	routes, err := w.Routes()
	if err != nil || len(routes) != 0 {
		t.Fatalf("skip became route: %v %v", routes, err)
	}
}

func TestSageSkippedAtomicFailureAndUnsafeFile(t *testing.T) {
	w := Writer{Root: t.TempDir()}
	if _, err := w.WriteSkipped("abcdef01"); err != nil {
		t.Fatal(err)
	}
	before := readTopic(t, w.Root, skippedFile)
	w.replace = func(string, []byte) error { return errors.New("injected failure") }
	if _, err := w.WriteSkipped("abcdef02"); err == nil {
		t.Fatal("expected failure")
	}
	if readTopic(t, w.Root, skippedFile) != before {
		t.Fatal("lost prior record")
	}
	if _, err := w.WriteSkipped("bad\n<!-- sig: abcdef -->"); err == nil {
		t.Fatal("accepted invalid signature")
	}
	w.Root = t.TempDir()
	if err := os.Mkdir(filepath.Join(w.Root, skippedFile), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteSkipped("abcdef02"); err == nil {
		t.Fatal("accepted unsafe file")
	}
}
