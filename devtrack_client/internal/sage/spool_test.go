package sage

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func spoolEvent(id string) Event {
	success, code := true, 0
	return Event{SchemaVersion: 1, EventID: id, Harness: "codex-history", SessionID: "session", EventType: "command", Tool: "shell", OccurredAt: time.Unix(10, 0).UTC(), Command: "git status", Signature: "git status", Success: &success, ExitCode: &code}
}

func TestConcurrentRepeatedDeliveryPublishesOneImmutableFile(t *testing.T) {
	root := t.TempDir()
	var group sync.WaitGroup
	errorsSeen := make(chan error, 20)
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsSeen <- WriteEvent(root, spoolEvent("same-delivery"))
		}()
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(pendingDir(root))
	if err != nil || len(entries) != 1 {
		t.Fatalf("pending files=%d err=%v", len(entries), err)
	}
}

func TestWriteAndImportSpoolIsAtomicBoundedAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"one", "two"} {
		if err := WriteEvent(root, spoolEvent(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteEvent(root, spoolEvent("one")); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	sink := func(event Event) (bool, error) {
		if seen[event.DeliveryKey()] {
			return false, nil
		}
		seen[event.DeliveryKey()] = true
		return true, nil
	}
	result, err := ImportSpool(root, 1, sink)
	if err != nil || result.Imported != 1 || result.Remaining != 1 {
		t.Fatalf("first import: %+v, %v", result, err)
	}
	result, err = ImportSpool(root, 10, sink)
	if err != nil || result.Imported != 1 || result.Remaining != 0 {
		t.Fatalf("second import: %+v, %v", result, err)
	}
}

func TestImportSpoolQuarantinesBadInputAndRetriesSinkFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(pendingDir(root), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pendingDir(root), "bad.json"), []byte(`{"command":"secret=canary"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteEvent(root, spoolEvent("retry")); err != nil {
		t.Fatal(err)
	}
	result, err := ImportSpool(root, 10, func(Event) (bool, error) { return false, errors.New("locked") })
	if err != nil || result.Quarantined != 1 || result.Remaining != 1 {
		t.Fatalf("result: %+v, %v", result, err)
	}
	entries, err := os.ReadDir(quarantineDir(root))
	if err != nil || len(entries) != 1 {
		t.Fatalf("quarantine: %d, %v", len(entries), err)
	}
}

func TestPausedSpoolDropsNewEventsAndDoesNotImportBacklog(t *testing.T) {
	root := t.TempDir()
	if err := WriteEvent(root, spoolEvent("before")); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	if err := WriteEvent(root, spoolEvent("during")); err != nil {
		t.Fatal(err)
	}
	result, err := ImportSpool(root, 10, func(Event) (bool, error) { t.Fatal("sink called while paused"); return false, nil })
	if err != nil || result != (ImportResult{}) {
		t.Fatalf("paused import: %+v, %v", result, err)
	}
}
