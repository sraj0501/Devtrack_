package sage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuarantineInspectionIsOpaqueBoundedAndPurgeIsExplicit(t *testing.T) {
	root := t.TempDir()
	dir := quarantineDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const canary = "PRIVATE-COMMAND-CANARY"
	if err := os.WriteFile(filepath.Join(dir, canary+".json"), []byte(`{"command":"`+canary+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	listed, err := ListQuarantine(root)
	if err != nil || listed.Total != 1 || len(listed.Entries) != 1 {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	entry := listed.Entries[0]
	if entry.Reason != "invalid_event" || len(entry.ID) != 16 || strings.Contains(entry.ID, "PRIVATE") {
		t.Fatalf("unsafe entry=%+v", entry)
	}
	if _, err := PurgeQuarantine(root, "bad", false); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if purged, err := PurgeQuarantine(root, entry.ID, false); err != nil || purged.Removed != 1 {
		t.Fatalf("purge=%+v err=%v", purged, err)
	}
	if _, err := PurgeQuarantine(root, entry.ID, false); err == nil {
		t.Fatal("missing entry did not fail")
	}
}

func TestQuarantineListCapsResultsAndNeverFollowsLinks(t *testing.T) {
	root := t.TempDir()
	dir := quarantineDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxQuarantineEntries+2; i++ {
		name := filepath.Join(dir, strings.Repeat("a", 8)+string(rune(0x1000+i))+".json")
		if err := os.WriteFile(name, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := ListQuarantine(root)
	if err != nil || listed.Total != MaxQuarantineEntries+2 || len(listed.Entries) != MaxQuarantineEntries || !listed.Truncated {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	if purged, err := PurgeQuarantine(root, "", true); err != nil || purged.Removed != MaxQuarantineEntries+2 {
		t.Fatalf("purge all=%+v err=%v", purged, err)
	}
}

func TestQuarantinePurgeRemovesSymlinkWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	dir := quarantineDir(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	linkName := "linked.json"
	link := filepath.Join(dir, linkName)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	listed, err := ListQuarantine(root)
	if err != nil || len(listed.Entries) != 1 || listed.Entries[0].Reason != "unsafe_file" {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	if purged, err := PurgeQuarantine(root, quarantineOpaqueID(linkName), false); err != nil || purged.Removed != 1 {
		t.Fatalf("purge=%+v err=%v", purged, err)
	}
	if raw, err := os.ReadFile(target); err != nil || string(raw) != "outside" {
		t.Fatalf("symlink target changed: %q err=%v", raw, err)
	}
}
