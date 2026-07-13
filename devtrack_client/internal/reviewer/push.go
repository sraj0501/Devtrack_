package reviewer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PushToRemote runs "git push origin HEAD:<branchName>" in repoPath.
// Returns error on non-zero exit or context cancellation.
func PushToRemote(ctx context.Context, repoPath, branchName string) error {
	if repoPath == "" {
		return fmt.Errorf("PushToRemote: repoPath is empty")
	}
	if branchName == "" {
		return fmt.Errorf("PushToRemote: branchName is empty")
	}
	cmd := exec.CommandContext(ctx, "git", "push", "origin", "HEAD:"+branchName)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PushToRemote(%q): %w — %s", branchName, err, strings.TrimSpace(string(out)))
	}
	return nil
}
