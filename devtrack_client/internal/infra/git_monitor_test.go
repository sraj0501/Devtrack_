package infra

import (
	"strings"
	"testing"
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
