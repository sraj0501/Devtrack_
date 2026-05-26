"""PR/MR finding utilities for git-sage."""
import re
from typing import Optional, List, Dict
from .context import run_git


class PRFinder:
    """Find and analyze pull/merge requests."""

    def __init__(self, cwd: str = "."):
        self.cwd = cwd

    def get_origin_remote(self) -> Optional[str]:
        """Get the origin remote URL."""
        output, ok = run_git(["remote", "get-url", "origin"], self.cwd)
        if ok and output:
            return output.strip()
        return None

    def extract_repo_info(self, remote_url: str) -> Optional[Dict[str, str]]:
        """Extract owner/repo from GitHub/GitLab/Bitbucket URL."""
        patterns = [
            # GitHub: git@github.com:owner/repo.git or https://github.com/owner/repo.git
            r"github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?$",
            # GitLab: git@gitlab.com:owner/repo.git or https://gitlab.com/owner/repo.git
            r"gitlab\.com[:/]([^/]+)/([^/]+?)(?:\.git)?$",
            # Bitbucket: git@bitbucket.org:owner/repo.git
            r"bitbucket\.org[:/]([^/]+)/([^/]+?)(?:\.git)?$",
        ]

        for pattern in patterns:
            match = re.search(pattern, remote_url)
            if match:
                return {
                    "provider": "github" if "github" in remote_url else (
                        "gitlab" if "gitlab" in remote_url else "bitbucket"
                    ),
                    "owner": match.group(1),
                    "repo": match.group(2)
                }
        return None

    def get_current_branch(self) -> Optional[str]:
        """Get current branch name."""
        output, ok = run_git(["rev-parse", "--abbrev-ref", "HEAD"], self.cwd)
        return output.strip() if ok else None

    def parse_pr_from_branch_name(self, branch_name: str) -> Optional[int]:
        """Try to extract PR number from branch name patterns.

        Common patterns:
        - pr/123 or pr-123
        - fix/issue-123 or feat/issue-123
        - release/v1.0.0
        """
        patterns = [
            r"pr[/-](\d+)",
            r"(?:fix|feat|feature|bugfix)/.*?[-#](\d+)",
            r"issue[-#](\d+)",
        ]

        for pattern in patterns:
            match = re.search(pattern, branch_name, re.IGNORECASE)
            if match:
                return int(match.group(1))
        return None

    def get_branch_description(self) -> Optional[str]:
        """Get the reflog description for current branch."""
        branch = self.get_current_branch()
        if not branch:
            return None

        output, ok = run_git(["reflog", "show", "-n", "1", branch], self.cwd)
        if ok and output:
            # Try to extract meaningful info
            return output.strip()
        return None

    def find_related_commits(self, pattern: str, max_commits: int = 100) -> List[Dict[str, str]]:
        """Find commits mentioning issue/PR number.

        Searches commit messages for patterns like:
        - fixes #123, closes #123
        - relates to #123
        - PR #123
        """
        output, ok = run_git(["log", f"-{max_commits}", "-i", f"--grep={pattern}", "--oneline"], self.cwd)
        if not ok or not output:
            return []

        commits = []
        for line in output.strip().split("\n"):
            if line:
                parts = line.split(None, 1)
                commits.append({
                    "sha": parts[0][:8],
                    "message": parts[1] if len(parts) > 1 else ""
                })
        return commits

    def suggest_pr_metadata(self) -> Dict[str, str]:
        """Suggest PR metadata based on current branch and commits."""
        branch = self.get_current_branch()
        if not branch:
            return {}

        metadata = {
            "branch": branch,
        }

        # Try to extract title from branch name
        branch_title = branch.replace("-", " ").replace("_", " ").title()
        metadata["suggested_title"] = branch_title

        # Try to find related issue number
        issue_num = self.parse_pr_from_branch_name(branch)
        if issue_num:
            metadata["issue_number"] = str(issue_num)

        # Get recent commits on this branch compared to main
        output, ok = run_git(["log", "main..HEAD", "--oneline"], self.cwd)
        if ok and output:
            lines = output.strip().split("\n")
            metadata["commits_count"] = str(len([l for l in lines if l]))

        return metadata

    def list_open_branches(self) -> List[Dict[str, str]]:
        """List branches that look like they have open PRs.

        Returns branches that:
        - Are not main/master/develop
        - Have recent commits
        - Seem to have meaningful names
        """
        output, ok = run_git(["branch", "-v"], self.cwd)
        if not ok:
            return []

        branches = []
        protected = {"main", "master", "develop", "development", "release"}

        for line in output.strip().split("\n"):
            if not line:
                continue

            # Format: * branch_name commit_hash message
            parts = line.strip().split(None, 2)
            if len(parts) < 2:
                continue

            branch_name = parts[0].lstrip("* ")
            if branch_name in protected:
                continue

            # Check if it's a meaningful name (has / or - and multiple words)
            if re.search(r"[/-]", branch_name) or len(branch_name.split("_")) > 1:
                branches.append({
                    "name": branch_name,
                    "sha": parts[1][:8] if len(parts) > 1 else "",
                    "msg": parts[2] if len(parts) > 2 else ""
                })

        return branches

    def get_diff_stats(self, branch1: str = "main", branch2: str = "HEAD") -> Dict[str, int]:
        """Get diff statistics between two branches."""
        output, ok = run_git(["diff", "--stat", f"{branch1}..{branch2}"], self.cwd)
        if not ok or not output:
            return {"files": 0, "additions": 0, "deletions": 0}

        # Last line has summary like "2 files changed, 5 insertions(+), 3 deletions(-)"
        lines = output.strip().split("\n")
        if not lines:
            return {"files": 0, "additions": 0, "deletions": 0}

        summary_line = lines[-1]
        files = 0
        additions = 0
        deletions = 0

        # Parse "X files changed, Y insertions(+), Z deletions(-)"
        files_match = re.search(r"(\d+)\s+files?", summary_line)
        if files_match:
            files = int(files_match.group(1))

        add_match = re.search(r"(\d+)\s+insertions?", summary_line)
        if add_match:
            additions = int(add_match.group(1))

        del_match = re.search(r"(\d+)\s+deletions?", summary_line)
        if del_match:
            deletions = int(del_match.group(1))

        return {
            "files": files,
            "additions": additions,
            "deletions": deletions
        }
