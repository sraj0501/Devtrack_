package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// Real Git writes continue during downstream processing; every hash must survive.
func TestMonitorEnumeratesBurstsDuringSlowCallback(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_NO_DEVTRACK=1", "GIT_CONFIG_GLOBAL="+filepath.Join(repo, "no-config"), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q", "-b", "feature/BASIC-101-burst")
	git("config", "user.name", "Capture Test")
	git("config", "user.email", "capture@example.invalid")
	commit := func(message string) {
		git("-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", message)
	}
	commit("baseline")
	gm, err := NewGitMonitor(repo)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Stop()
	head, _ := gm.repo.Head()
	gm.seen = map[plumbing.Hash]bool{head.Hash(): true}
	for i := 0; i < 5; i++ {
		commit("first burst")
	}
	var got []CommitInfo
	slow := func(c CommitInfo) {
		got = append(got, c)
		if len(got) == 1 {
			// Simulates native Git running while an HTTP request is outstanding.
			for i := 0; i < 5; i++ {
				commit("second burst")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	gm.checkNewCommits(slow)
	gm.checkNewCommits(slow)
	gm.checkNewCommits(slow)
	if len(got) != 10 {
		t.Fatalf("captured %d of 10 commits", len(got))
	}
	hashes := map[string]bool{}
	for i, c := range got {
		if hashes[c.Hash] {
			t.Fatalf("duplicate %s", c.Hash)
		}
		hashes[c.Hash] = true
		if c.Branch != "feature/BASIC-101-burst" {
			t.Fatalf("branch=%q", c.Branch)
		}
		expected := "first burst"
		if i >= 5 {
			expected = "second burst"
		}
		if c.Message != expected {
			t.Fatalf("out of order at %d: %s", i, c.Message)
		}
	}
	// Revisiting known branches must not duplicate captured hashes.
	git("checkout", "--quiet", "HEAD~3")
	gm.checkNewCommits(slow)
	if len(got) != 10 {
		t.Fatalf("checkout duplicated history: %d", len(got))
	}
	// Git may switch away again before a slow HTTP callback returns. The
	// departed branch must still be scanned with its own ticket provenance.
	git("checkout", "--quiet", "-b", "feature/BASIC-102-switched")
	commit("switched branch work")
	commit("switched branch tests")
	git("checkout", "--quiet", "feature/BASIC-101-burst")
	gm.checkNewCommits(slow)
	gm.checkNewCommits(slow)
	if len(got) != 12 {
		t.Fatalf("lost switched-branch commits: %d", len(got))
	}
	for _, c := range got[10:] {
		if c.Branch != "feature/BASIC-102-switched" {
			t.Fatalf("wrong branch context: %+v", c)
		}
	}
}

func TestMonitorCapturesAllFirstCommitsInEmptyRepo(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-q", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	gm, err := NewGitMonitor(repo)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Stop()
	gm.seen = make(map[plumbing.Hash]bool)
	for i := 0; i < 3; i++ {
		cmd = exec.Command("git", "-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "new repo burst")
		cmd.Env = append(os.Environ(), "GIT_NO_DEVTRACK=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", out, err)
		}
	}
	count := 0
	gm.checkNewCommits(func(CommitInfo) { count++ })
	if count != 3 {
		t.Fatalf("captured %d of 3 initial commits", count)
	}
}
