package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// rawGit returns git stdout verbatim (no trimming) — required for patches.
func rawGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// numbered builds a deterministic multi-line file so we can drift specific lines.
func numbered(lines int, replace map[int]string) string {
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		if r, ok := replace[i]; ok {
			b.WriteString(r)
		} else {
			b.WriteString("line ")
			b.WriteByte(byte('0' + i%10))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// TestDeferredApplySurvivesTreeDrift is the core robustness test: a staged
// change is captured, the surrounding context then drifts (so a plain
// `git apply` would fail), and the 3-way apply still lands the change.
func TestDeferredApplySurvivesTreeDrift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "code.txt")

	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@t.test")
	runGit(t, dir, "config", "user.name", "Test")

	// Initial commit: 20 numbered lines.
	writeFile(t, file, numbered(20, nil))
	runGit(t, dir, "add", "code.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")

	// Stage a change on line 10, then snapshot it as the deferred change.
	writeFile(t, file, numbered(20, map[int]string{10: "STAGED CHANGE on ten"}))
	runGit(t, dir, "add", "code.txt")
	patch := rawGit(t, dir, "diff", "--cached") // raw: patches must keep exact bytes/newlines
	base, snap := captureSnapshot(dir, "queued: change line 10")
	if snap == "" {
		t.Fatal("captureSnapshot returned empty snapshot SHA")
	}
	if base == "" {
		t.Fatal("captureSnapshot returned empty base SHA")
	}

	// Simulate the tree moving on: unstage, then drift the *context* (line 8)
	// and commit it, so the queued hunk's context no longer matches.
	runGit(t, dir, "reset", "-q")
	writeFile(t, file, numbered(20, map[int]string{8: "DRIFTED context on eight"}))
	runGit(t, dir, "add", "code.txt")
	runGit(t, dir, "commit", "-q", "-m", "drift line 8")

	// A plain apply must now fail (proving the drift is real)...
	plain := exec.Command("git", "apply", "--cached", "--check", "-")
	plain.Dir = dir
	plain.Stdin = strings.NewReader(patch)
	if err := plain.Run(); err == nil {
		t.Fatal("expected plain git apply to fail after context drift, but it succeeded")
	}

	// ...while the 3-way apply succeeds using the snapshot's pinned blobs.
	if err := gitApply3way(dir, patch); err != nil {
		t.Fatalf("3-way apply should survive context drift: %v", err)
	}

	got, _ := os.ReadFile(file)
	if !strings.Contains(string(got), "STAGED CHANGE on ten") {
		t.Error("queued line-10 change was not applied")
	}
	if !strings.Contains(string(got), "DRIFTED context on eight") {
		t.Error("drifted line-8 change was clobbered")
	}
}

// TestSnapshotRefLifecycle verifies a queued snapshot is pinned by a ref and
// that pruning it (as expiry does) removes the ref cleanly.
func TestSnapshotRefLifecycle(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "code.txt")

	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@t.test")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, file, numbered(10, nil))
	runGit(t, dir, "add", "code.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")

	// Stage a change and capture + pin a snapshot, as QueueCommit does.
	writeFile(t, file, numbered(10, map[int]string{5: "snapshot me"}))
	runGit(t, dir, "add", "code.txt")
	_, snap := captureSnapshot(dir, "queued")
	if snap == "" {
		t.Fatal("expected a snapshot SHA")
	}
	ref := snapshotRef(1)
	runGit(t, dir, "update-ref", ref, snap)

	// The ref must exist and resolve to the snapshot.
	if got := runGit(t, dir, "rev-parse", ref); got != snap {
		t.Fatalf("pinned ref = %s, want snapshot %s", got, snap)
	}

	// Prune it (what expiry does) and confirm it is gone.
	if _, err := gitOut(dir, "update-ref", "-d", ref); err != nil {
		t.Fatalf("pruning the ref failed: %v", err)
	}
	verify := exec.Command("git", "show-ref", "--verify", ref)
	verify.Dir = dir
	if verify.Run() == nil {
		t.Error("snapshot ref still exists after pruning")
	}
}

// TestPatchAlreadyApplied verifies the already-applied fast-path detection in
// the no-drift case (the user committed the exact queued change elsewhere).
func TestPatchAlreadyApplied(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "code.txt")

	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "t@t.test")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, file, numbered(20, nil))
	runGit(t, dir, "add", "code.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")

	// Capture a staged change as a patch, then commit that exact change.
	writeFile(t, file, numbered(20, map[int]string{10: "applied change"}))
	runGit(t, dir, "add", "code.txt")
	patch := rawGit(t, dir, "diff", "--cached")
	runGit(t, dir, "commit", "-q", "-m", "apply it")

	if !patchAlreadyApplied(dir, patch) {
		t.Error("patchAlreadyApplied should be true once the change is committed")
	}

	// A fresh, unrelated patch must not be reported as applied.
	other := t.TempDir()
	runGit(t, other, "init", "-q")
	runGit(t, other, "config", "user.email", "t@t.test")
	runGit(t, other, "config", "user.name", "Test")
	writeFile(t, filepath.Join(other, "code.txt"), numbered(20, nil))
	runGit(t, other, "add", "code.txt")
	runGit(t, other, "commit", "-q", "-m", "init")
	if patchAlreadyApplied(other, patch) {
		t.Error("patchAlreadyApplied should be false for an unapplied change")
	}
}
