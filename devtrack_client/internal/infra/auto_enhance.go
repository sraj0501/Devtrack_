package infra

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/gitsage"
)

// autoEnhanceEnabled reports whether background commit-message enhancement is
// opted into via DEVTRACK_AUTO_ENHANCE=true (or "1" / "yes").
func autoEnhanceEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DEVTRACK_AUTO_ENHANCE")))
	return v == "true" || v == "1" || v == "yes"
}

// isCommitPushed returns true when the given commit hash is reachable from any
// remote-tracking ref. We refuse to amend a pushed commit because it would
// require a force-push.
func isCommitPushed(repoPath, hash string) bool {
	out, err := exec.Command("git", "-C", repoPath, "branch", "-r", "--contains", hash).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// hasStagedChanges returns true when the index contains changes that are not
// yet committed. Amending with staged changes would alter the commit's tree,
// which is not what we want here.
func hasStagedChanges(repoPath string) bool {
	out, err := exec.Command("git", "-C", repoPath, "diff", "--cached", "--name-only").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// tryAutoEnhance attempts to improve the message of the given commit using the
// configured LLM. On success it amends the commit and returns the new HEAD
// hash and the enhanced message. On any failure or skip it returns ("", "", false).
//
// Amending is skipped when:
//   - DEVTRACK_AUTO_ENHANCE is not enabled
//   - the LLM is unreachable
//   - the commit has already been pushed to a remote
//   - there are staged changes (to avoid unintended tree modifications)
//   - the LLM produces an identical or empty message
func tryAutoEnhance(repoPath string, commit CommitInfo) (newHash, newMsg string, ok bool) {
	if !autoEnhanceEnabled() {
		return "", "", false
	}
	if !gitsage.LLMReachable() {
		return "", "", false
	}
	if isCommitPushed(repoPath, commit.Hash) {
		log.Printf("[auto-enhance] skipping pushed commit %s", commit.Hash[:8])
		return "", "", false
	}
	if hasStagedChanges(repoPath) {
		log.Printf("[auto-enhance] skipping: staged changes present in %s", repoPath)
		return "", "", false
	}

	// Get the full diff of this commit for LLM context.
	diffOut, err := exec.Command("git", "-C", repoPath, "show", "--no-color", commit.Hash).Output()
	if err != nil {
		log.Printf("[auto-enhance] could not read diff for %s: %v", commit.Hash[:8], err)
		return "", "", false
	}

	enhanced, err := gitsage.EnhanceForDiff(commit.Message, string(diffOut), commit.Branch)
	if err != nil {
		log.Printf("[auto-enhance] LLM error for %s: %v", commit.Hash[:8], err)
		return "", "", false
	}
	enhanced = strings.TrimSpace(enhanced)
	if enhanced == "" || enhanced == strings.TrimSpace(commit.Message) {
		return "", "", false
	}

	// Amend only the message; the tree stays identical (no staged changes).
	if err := exec.Command("git", "-C", repoPath, "commit", "--amend", "-m", enhanced).Run(); err != nil {
		log.Printf("[auto-enhance] amend failed for %s: %v", commit.Hash[:8], err)
		return "", "", false
	}

	// Capture the new commit hash.
	rawHash, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", "", false
	}
	newHash = strings.TrimSpace(string(rawHash))

	subject := strings.SplitN(enhanced, "\n", 2)[0]
	log.Printf("[auto-enhance] %s → %s", commit.Hash[:8], newHash[:8])
	log.Printf("[auto-enhance]   original : %s", commit.Message)
	log.Printf("[auto-enhance]   enhanced : %s", subject)

	return newHash, enhanced, true
}
