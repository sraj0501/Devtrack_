package infra

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInstalledPostCommitHookIsSilentAndFailOpen(t *testing.T) {
	content := postCommitHookContent("/tmp/devtrack/commit.log")
	if !strings.Contains(content, "2>/dev/null || true") {
		t.Fatal("post-commit notification must suppress errors and fail open")
	}
	if !strings.Contains(content, "exit 0") {
		t.Fatal("post-commit hook must never fail the Git commit")
	}
}

// Execute the actual generated hook through Git. A text assertion alone cannot
// prove shell redirection failures are silent or that terminal input is unused.
func TestPostCommitRealGitSilentFailOpen(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		if terminal && runtime.GOOS != "linux" {
			continue // Linux CI supplies util-linux script for a real pseudo-terminal.
		}
		for _, available := range []bool{true, false} {
			name := "pipe"
			if terminal {
				name = "pty"
			}
			if available {
				name += "/writable"
			} else {
				name += "/unavailable"
			}
			t.Run(name, func(t *testing.T) {
				repo := t.TempDir()
				env := append(os.Environ(), "GIT_NO_DEVTRACK=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(repo, "no-global-config"), "GIT_TERMINAL_PROMPT=0")
				run := func(args ...string) string {
					t.Helper()
					cmd := exec.Command("git", args...)
					cmd.Dir, cmd.Env = repo, env
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("git %v: %v: %s", args, err, out)
					}
					return string(out)
				}
				run("init", "-q")
				run("config", "user.name", "Silence Test")
				run("config", "user.email", "silence@example.invalid")
				logPath := filepath.Join(repo, "logs with spaces", "commit.log")
				if available {
					if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
						t.Fatal(err)
					}
				}
				// Git for Windows executes hooks in its POSIX shell.
				shellPath := filepath.ToSlash(logPath)
				if runtime.GOOS == "windows" {
					shellPath = "/" + strings.ToLower(shellPath[:1]) + shellPath[2:]
				}
				if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-commit"), []byte(postCommitHookContent(shellPath)), 0755); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				args := []string{"-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "silence regression"}
				cmd := exec.CommandContext(ctx, "git", args...)
				if terminal {
					// Assert all three streams really are terminals before executing Git.
					cmd = exec.CommandContext(ctx, "script", "--quiet", "--return", "--command", "test -t 0 && test -t 1 && test -t 2 && exec git -c commit.gpgsign=false commit --quiet --allow-empty -m 'silence regression'", "/dev/null")
				}
				cmd.Dir, cmd.Env = repo, env
				out, err := cmd.CombinedOutput()
				if err != nil || len(out) != 0 {
					t.Fatalf("commit must succeed silently: err=%v output=%q", err, out)
				}
				if got := strings.TrimSpace(run("rev-list", "--count", "HEAD")); got != "1" {
					t.Fatalf("commit count=%s", got)
				}
				if available {
					data, err := os.ReadFile(logPath)
					if err != nil || !strings.Contains(string(data), "Commit detected at") {
						t.Fatalf("observation missing: %q, %v", data, err)
					}
				} else if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Fatalf("unavailable destination unexpectedly exists: %v", err)
				}
			})
		}
	}
}
