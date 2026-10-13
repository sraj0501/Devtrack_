package gitcommit_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge/gitcommit"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Sage Test")
	git(t, root, "config", "user.email", "sage@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "seed.txt")
	git(t, root, "commit", "-q", "-m", "seed")
	return root
}

func tracked(t *testing.T, root, name string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "--", name)
	git(t, root, "commit", "-q", "-m", "track "+name)
}

func commit(t *testing.T, root string, enabled bool, changes ...gitcommit.Change) bool {
	t.Helper()
	committed, err := (gitcommit.Committer{KnowledgeRoot: root, Enabled: enabled}).Commit(context.Background(), changes)
	if err != nil {
		t.Fatal(err)
	}
	return committed
}

func TestReferenceNestedKnowledgeFolderCommitsWithRootRelativePaths(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "docs", "personal knowledge")
	path := filepath.Join(root, "git.md")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("# Git\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("expected knowledge commit")
	}
	if got := git(t, repo, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(filepath.ToSlash(got), "docs/personal knowledge/git.md") {
		t.Fatalf("committed paths=%q", got)
	}
}

func TestReferencePreexistingWorkIsLeftUncommitted(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("user edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path, raw := filepath.Join(root, "git.md"), []byte("# Git\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("expected knowledge commit")
	}
	if got := git(t, repo, "status", "--porcelain"); !strings.Contains(got, "seed.txt") {
		t.Fatalf("user edit lost: %q", got)
	}
}

func TestReferenceEntangledFileIsSkippedNotGuessed(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	tracked(t, repo, "knowledge/git.md", []byte("# Git\n"))
	path := filepath.Join(root, "git.md")
	before, after := []byte("# Git\nuser note\n"), []byte("# Git\nuser note\nSage entry\n")
	if err := os.WriteFile(path, after, 0600); err != nil {
		t.Fatal(err)
	}
	if commit(t, root, true, gitcommit.Change{Path: path, Before: before, After: after}) {
		t.Fatal("entangled file was committed")
	}
	if got := git(t, repo, "status", "--porcelain"); !strings.Contains(got, "knowledge/git.md") {
		t.Fatalf("entangled edit disappeared: %q", got)
	}
}

func TestReferenceUserStagedWorkStaysStaged(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("staged user edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "seed.txt")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path, raw := filepath.Join(root, "git.md"), []byte("# Git\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("expected knowledge commit")
	}
	if got := git(t, repo, "diff", "--cached", "--name-only"); strings.TrimSpace(got) != "seed.txt" {
		t.Fatalf("staged work changed: %q", got)
	}
}

func TestReferencePathsWithSpacesSurvive(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge base")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path, raw := filepath.Join(root, "shell tips.md"), []byte("# Shell\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("path with spaces was not committed")
	}
}

func TestReferenceDeletionByWriterIsCommitted(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	before := []byte("# Old topic\n")
	tracked(t, repo, "knowledge/old.md", before)
	path := filepath.Join(root, "old.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, Before: before}) {
		t.Fatal("owned deletion was not committed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted file restored: %v", err)
	}
}

func TestExistingExecutableModeIsPreserved(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	before, after := []byte("# Tool\n"), []byte("# Tool\n\nUpdated.\n")
	tracked(t, repo, "knowledge/tool.md", before)
	git(t, repo, "update-index", "--chmod=+x", "knowledge/tool.md")
	git(t, repo, "commit", "-q", "-m", "mark executable")
	path := filepath.Join(root, "tool.md")
	if err := os.WriteFile(path, after, 0600); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, gitcommit.Change{Path: path, Before: before, After: after}) {
		t.Fatal("expected knowledge commit")
	}
	if got := git(t, repo, "ls-tree", "HEAD", "--", "knowledge/tool.md"); !strings.HasPrefix(got, "100755 ") {
		t.Fatalf("executable mode changed: %q", got)
	}
}

func TestReferenceNothingOfOursMakesNoCommit(t *testing.T) {
	repo := repository(t)
	before := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))
	if commit(t, repo, true) {
		t.Fatal("empty commit created")
	}
	after := strings.TrimSpace(git(t, repo, "rev-parse", "HEAD"))
	if before != after {
		t.Fatalf("HEAD changed: %s -> %s", before, after)
	}
}

func TestReferenceAutoCommitOffIsRespected(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path, raw := filepath.Join(root, "git.md"), []byte("# Git\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if commit(t, root, false, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("disabled commit was created")
	}
}

func TestReferenceNonRepoIsANoOp(t *testing.T) {
	root := t.TempDir()
	path, raw := filepath.Join(root, "git.md"), []byte("# Git\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if commit(t, root, true, gitcommit.Change{Path: path, After: raw}) {
		t.Fatal("non-repository commit created")
	}
}

func TestReferenceGitignoredRuntimeIsNeverDirty(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	recorder := &gitcommit.Recorder{}
	writer := knowledge.Writer{Root: root, Recorder: recorder}
	if err := writer.RememberRoute("git", "version-control"); err != nil {
		t.Fatal(err)
	}
	if !commit(t, root, true, recorder.Changes()...) {
		t.Fatal("gitignore was not committed")
	}
	if got := git(t, repo, "status", "--porcelain", "--untracked-files=all"); strings.Contains(got, ".routes.json") || strings.Contains(got, ".merge.json") {
		t.Fatalf("runtime dirtied repository: %q", got)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), ".routes.json") {
		t.Fatalf("gitignore=%q err=%v", ignore, err)
	}
}

func TestReferenceNewKnowledgeBaseNeverTracksTheQueue(t *testing.T) {
	TestReferenceGitignoredRuntimeIsNeverDirty(t)
}

func TestReferenceExistingGitignoreIsExtendedNotReplaced(t *testing.T) {
	repo := repository(t)
	root := filepath.Join(repo, "knowledge")
	tracked(t, repo, "knowledge/.gitignore", []byte("keep-this-rule\n"))
	recorder := &gitcommit.Recorder{}
	writer := knowledge.Writer{Root: root, Recorder: recorder}
	if err := writer.RememberRoute("git", "version-control"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "keep-this-rule") || !strings.Contains(text, ".routes.json") {
		t.Fatalf("gitignore was replaced: %q", text)
	}
}
