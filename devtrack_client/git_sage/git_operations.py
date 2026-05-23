"""Advanced git operations and utilities."""
import subprocess
import json
import re
from typing import Optional, List, Dict, Tuple
from .context import run_git


class GitOperations:
    """High-level git operations helper."""

    def __init__(self, cwd: str = "."):
        self.cwd = cwd

    # ── Branch operations ────────────────────────────────────────────────────

    def list_branches(self, remote: bool = False) -> List[str]:
        """List all local or remote branches."""
        args = ["branch", "-r" if remote else ""]
        output, ok = run_git([arg for arg in args if arg], self.cwd)
        if not ok:
            return []
        return [line.strip().lstrip("* ") for line in output.split("\n") if line.strip()]

    def get_current_branch(self) -> str:
        """Get current branch name."""
        output, ok = run_git(["rev-parse", "--abbrev-ref", "HEAD"], self.cwd)
        return output if ok else "unknown"

    def branch_exists(self, branch: str) -> bool:
        """Check if branch exists (local or remote)."""
        output, ok = run_git(["rev-parse", "--verify", branch], self.cwd)
        return ok

    def delete_merged_branches(self, base: str = "main") -> List[str]:
        """Find branches merged into base and return them for deletion."""
        output, ok = run_git(["branch", "--merged", base], self.cwd)
        if not ok:
            return []
        branches = [line.strip().lstrip("* ") for line in output.split("\n") if line.strip()]
        # Filter out protected branches
        protected = {base, "main", "develop", "development", "master"}
        return [b for b in branches if b not in protected]

    # ── Commit operations ────────────────────────────────────────────────────

    def get_commit_log(self, n: int = 10, branch: Optional[str] = None) -> List[Dict[str, str]]:
        """Get recent commits as list of dicts with sha, author, message."""
        args = ["log", f"-{n}", "--pretty=format:%H|%an|%s"]
        if branch:
            args.append(branch)
        output, ok = run_git(args, self.cwd)
        if not ok:
            return []
        commits = []
        for line in output.strip().split("\n"):
            if "|" in line:
                parts = line.split("|", 2)
                commits.append({
                    "sha": parts[0][:8],
                    "author": parts[1] if len(parts) > 1 else "",
                    "message": parts[2] if len(parts) > 2 else ""
                })
        return commits

    def get_commit_diff(self, commit: str) -> str:
        """Get diff for a specific commit."""
        output, ok = run_git(["show", commit], self.cwd)
        return output if ok else ""

    def find_commit_by_message(self, pattern: str, max_commits: int = 50) -> Optional[str]:
        """Find commit by message pattern (regex)."""
        output, ok = run_git(["log", f"-{max_commits}", "--oneline"], self.cwd)
        if not ok:
            return None
        for line in output.split("\n"):
            if re.search(pattern, line, re.IGNORECASE):
                return line.split()[0]
        return None

    # ── Merge/Rebase operations ──────────────────────────────────────────────

    def detect_conflicts(self) -> List[str]:
        """Detect conflicted files in current state."""
        output, ok = run_git(["diff", "--name-only", "--diff-filter=U"], self.cwd)
        if not ok:
            return []
        return [f.strip() for f in output.split("\n") if f.strip()]

    def read_conflict_file(self, path: str) -> str:
        """Read a file with conflict markers."""
        try:
            with open(f"{self.cwd}/{path}", "r") as f:
                return f.read()
        except Exception as e:
            return f"Error reading {path}: {e}"

    def resolve_conflict_markers(self, content: str, strategy: str = "both") -> str:
        """
        Attempt to resolve conflict markers.
        Strategies: 'ours', 'theirs', 'both' (merge both sides), 'auto' (heuristic)
        """
        # Pattern: <<<<<<< HEAD\n...\n=======\n...\n>>>>>>> branch
        pattern = r'<<<<<<< HEAD\n(.*?)\n=======\n(.*?)\n>>>>>>> (.+?)\n'

        def resolve(match):
            ours = match.group(1)
            theirs = match.group(2)
            branch = match.group(3)

            if strategy == "ours":
                return ours
            elif strategy == "theirs":
                return theirs
            elif strategy == "both":
                # Merge both sides: try to keep both if they don't contradict
                return f"{ours}\n{theirs}"
            elif strategy == "auto":
                # Heuristic: prefer shorter non-whitespace version
                ours_clean = ours.strip()
                theirs_clean = theirs.strip()
                if len(ours_clean) == 0:
                    return theirs
                elif len(theirs_clean) == 0:
                    return ours
                else:
                    # Both have content, keep both
                    return f"{ours_clean}\n{theirs_clean}"
            return match.group(0)

        return re.sub(pattern, resolve, content, flags=re.MULTILINE | re.DOTALL)

    # ── Pull/Push operations ─────────────────────────────────────────────────

    def get_remote_branches(self) -> List[str]:
        """Get list of remote branches."""
        return self.list_branches(remote=True)

    def check_tracking_branch(self) -> Optional[str]:
        """Get the tracking branch for current branch."""
        output, ok = run_git(["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"], self.cwd)
        if ok and output != "@{u}":
            return output
        return None

    def get_ahead_behind(self, branch: str = "@{u}") -> Tuple[int, int]:
        """Get commits ahead and behind tracking branch."""
        ahead, _ = run_git(["rev-list", "--count", f"{branch}..HEAD"], self.cwd)
        behind, _ = run_git(["rev-list", "--count", f"HEAD..{branch}"], self.cwd)
        try:
            return int(ahead), int(behind)
        except ValueError:
            return 0, 0

    # ── Stash operations ─────────────────────────────────────────────────────

    def list_stashes(self) -> List[Dict[str, str]]:
        """List all stashed changes."""
        output, ok = run_git(["stash", "list"], self.cwd)
        if not ok:
            return []
        stashes = []
        for line in output.strip().split("\n"):
            if line:
                # Format: stash@{0}: message
                match = re.match(r"(stash@\{\d+\}): (.+)", line)
                if match:
                    stashes.append({
                        "ref": match.group(1),
                        "message": match.group(2)
                    })
        return stashes

    # ── Status operations ────────────────────────────────────────────────────

    def is_clean(self) -> bool:
        """Check if working tree is clean."""
        output, ok = run_git(["status", "--porcelain"], self.cwd)
        return ok and not output.strip()

    def get_status_files(self) -> Dict[str, List[str]]:
        """Get modified, added, deleted files."""
        output, ok = run_git(["status", "--porcelain"], self.cwd)
        if not ok:
            return {"modified": [], "added": [], "deleted": []}

        status = {"modified": [], "added": [], "deleted": []}
        for line in output.strip().split("\n"):
            if not line:
                continue
            code = line[:2]
            path = line[3:]
            if "M" in code:
                status["modified"].append(path)
            elif "A" in code:
                status["added"].append(path)
            elif "D" in code:
                status["deleted"].append(path)
        return status

    # ── Blame and history ────────────────────────────────────────────────────

    def blame_file(self, path: str) -> str:
        """Get blame for a file."""
        output, ok = run_git(["blame", path], self.cwd)
        return output if ok else f"Error blaming {path}"

    def find_who_deleted(self, path: str) -> Optional[str]:
        """Find who deleted a file."""
        output, ok = run_git(["log", "--diff-filter=D", "--summary", "-S", path], self.cwd)
        if ok and output:
            # Extract first commit
            for line in output.split("\n"):
                if line.startswith("commit"):
                    return line.split()[1][:8]
        return None

    # ── Utility operations ───────────────────────────────────────────────────

    def estimate_merge_complexity(self, branch1: str, branch2: str) -> Dict[str, int]:
        """Estimate how many conflicts might arise from merging."""
        # Count commits in each branch
        out1, _ = run_git(["rev-list", "--count", f"{branch1}..{branch2}"], self.cwd)
        out2, _ = run_git(["rev-list", "--count", f"{branch2}..{branch1}"], self.cwd)

        # Count files that changed in both
        common_files, _ = run_git([
            "diff", "--name-only", f"{branch1}...{branch2}"
        ], self.cwd)

        return {
            "branch2_commits_ahead": int(out1) if out1.isdigit() else 0,
            "branch1_commits_ahead": int(out2) if out2.isdigit() else 0,
            "files_changed": len(common_files.strip().split("\n")) if common_files.strip() else 0
        }

    def suggest_merge_base(self, branch1: str, branch2: str) -> str:
        """Get merge base between two branches."""
        output, ok = run_git(["merge-base", branch1, branch2], self.cwd)
        return output if ok else "unknown"
