package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestReferenceLockIsExclusiveAndReleasable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devtrack.lock")
	first, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := acquireDaemonLock(path); err == nil {
		releaseDaemonLock(second)
		t.Fatal("second daemon lock was acquired while the first was held")
	}
	releaseDaemonLock(first)
	second, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("released daemon lock was not reusable: %v", err)
	}
	releaseDaemonLock(second)
}

func TestReferenceStaleLockIsReclaimed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devtrack.lock")
	if err := os.WriteFile(path, []byte("stale artifact"), 0600); err != nil {
		t.Fatal(err)
	}
	ancient := time.Unix(1, 0)
	if err := os.Chtimes(path, ancient, ancient); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("an unlocked stale artifact blocked the process-owned lock: %v", err)
	}
	releaseDaemonLock(lock)
}

func TestReferenceDeadPidIsNotAlive(t *testing.T) {
	if CheckProcessAlive(1 << 30) {
		t.Fatal("impossible PID reported alive")
	}
}

func TestReferenceMissingPidfileIsNotAlive(t *testing.T) {
	d := &Daemon{pidFile: filepath.Join(t.TempDir(), "missing.pid")}
	if d.IsRunning() {
		t.Fatal("daemon without a PID file reported alive")
	}
}

func TestReferenceOwnPidWithFreshTouchIsAlive(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "devtrack.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(pidFile, now, now); err != nil {
		t.Fatal(err)
	}
	d := &Daemon{pidFile: pidFile}
	if !d.IsRunning() {
		t.Fatal("current process with a fresh PID record reported dead")
	}
}
