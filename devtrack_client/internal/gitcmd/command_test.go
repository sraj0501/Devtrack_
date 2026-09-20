package gitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCommitArgsSeparatesDevTrackAndGitFlags(t *testing.T) {
	flags := parseCommitArgs([]string{
		"--message=fix: keep queue state",
		"--dry-run",
		"--no-enhance",
		"--all",
		"--amend",
		"--signoff",
	})

	if flags.message != "fix: keep queue state" || !flags.dryRun || !flags.noEnhance || !flags.all || !flags.amend {
		t.Fatalf("unexpected parsed flags: %+v", flags)
	}
	if got := strings.Join(flags.passthru, " "); got != "--all --amend --signoff" {
		t.Fatalf("passthru = %q", got)
	}
}

func TestLoadConfigUsesSharedProviderAndIgnoresLegacySageVariables(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "ollama")
	t.Setenv("OLLAMA_HOST", "127.0.0.1:11434")
	t.Setenv("OLLAMA_MODEL", "shared-model")
	t.Setenv("GIT_SAGE_PROVIDER", "openai")
	t.Setenv("GIT_SAGE_DEFAULT_MODEL", "legacy-model")

	cfg := LoadConfig()
	if cfg.Provider != ProviderOllama || cfg.LLM.Model != "shared-model" {
		t.Fatalf("LoadConfig() = %+v", cfg)
	}
	if cfg.LLM.Host != "http://127.0.0.1:11434" {
		t.Fatalf("host = %q", cfg.LLM.Host)
	}
}

func TestGitOpsReadsStagedState(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	path := filepath.Join(repo, "file.txt")
	if err := os.WriteFile(path, []byte("content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "file.txt")

	git := NewGitOps(repo)
	if !git.IsRepo() {
		t.Fatal("IsRepo() = false")
	}
	files, err := git.StagedFiles()
	if err != nil || len(files) != 1 || files[0] != "file.txt" {
		t.Fatalf("StagedFiles() = %q, %v", files, err)
	}
	diff, err := git.DiffCached()
	if err != nil || !strings.Contains(diff, "+content") {
		t.Fatalf("DiffCached() = %q, %v", diff, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=commit.gpgsign", "GIT_CONFIG_VALUE_0=false")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
