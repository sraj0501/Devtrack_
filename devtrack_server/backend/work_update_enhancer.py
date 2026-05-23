"""
Work Update Enhancer - Enriches work update prompts with git context

This module uses git-sage's PRFinder to add intelligent context to work update
prompts, including:
- Current branch information
- PR/issue metadata
- Related commits
- Diff statistics
- Change summary

This helps the NLP parser extract better task information and provides better
context for AI enhancement.
"""

import logging
from typing import Optional, Dict, List

logger = logging.getLogger(__name__)

try:
    from backend.git_sage import PRFinder, GitOperations
    HAS_GIT_SAGE = True
except ImportError:
    HAS_GIT_SAGE = False


class WorkUpdateEnhancer:
    """Enhance work updates with git context"""

    def __init__(self, repo_path: str = "."):
        self.repo_path = repo_path
        self.pr_finder = PRFinder(cwd=repo_path) if HAS_GIT_SAGE else None
        self.git_ops = GitOperations(cwd=repo_path) if HAS_GIT_SAGE else None

    def get_branch_context(self) -> Optional[Dict[str, str]]:
        """Get branch and PR context"""
        if not self.pr_finder:
            return None

        try:
            branch = self.pr_finder.get_current_branch()
            if not branch:
                return None

            metadata = self.pr_finder.suggest_pr_metadata()

            context = {
                "branch": branch,
                "issue_number": metadata.get("issue_number"),
                "suggested_title": metadata.get("suggested_title"),
                "commits_count": metadata.get("commits_count")
            }

            return {k: v for k, v in context.items() if v}
        except Exception as e:
            logger.debug(f"Error getting branch context: {e}")
            return None

    def get_change_context(self) -> Optional[Dict[str, any]]:
        """Get context about what changed"""
        if not self.pr_finder:
            return None

        try:
            stats = self.pr_finder.get_diff_stats()

            return {
                "files_changed": stats.get("files", 0),
                "additions": stats.get("additions", 0),
                "deletions": stats.get("deletions", 0),
            }
        except Exception as e:
            logger.debug(f"Error getting change context: {e}")
            return None

    def get_related_work(self) -> Optional[List[Dict[str, str]]]:
        """Get related commits and PRs"""
        if not self.git_ops or not self.pr_finder:
            return None

        try:
            branch = self.pr_finder.get_current_branch()
            if not branch:
                return None

            # Get recent commits on this branch
            commits = self.git_ops.get_commit_log(n=5)
            related = []

            for commit in commits:
                related.append({
                    "sha": commit["sha"],
                    "message": commit["message"]
                })

            return related if related else None
        except Exception as e:
            logger.debug(f"Error getting related work: {e}")
            return None

    def format_work_update_context(self) -> str:
        """
        Format a human-readable work update context string.

        Example output:
        "Branch: feature-auth (PR #123)
         Changes: 5 files, +42 additions, -15 deletions
         Related: Fixed auth bug, Added token validation"
        """
        lines = []

        # Branch context
        branch_ctx = self.get_branch_context()
        if branch_ctx:
            branch_info = branch_ctx.get("branch", "unknown")
            if branch_ctx.get("issue_number"):
                branch_info += f" (#{branch_ctx['issue_number']})"
            lines.append(f"Branch: {branch_info}")

        # Change context
        change_ctx = self.get_change_context()
        if change_ctx:
            files = change_ctx.get("files_changed", 0)
            additions = change_ctx.get("additions", 0)
            deletions = change_ctx.get("deletions", 0)
            lines.append(f"Changes: {files} files, +{additions} -{deletions}")

        # Related work
        related = self.get_related_work()
        if related:
            related_msgs = [r["message"][:50] for r in related[:3]]
            lines.append(f"Related: {', '.join(related_msgs)}")

        return "\n".join(lines) if lines else None

    def enhance_prompt(self, base_prompt: str) -> str:
        """
        Enhance a work update prompt with git context.

        Args:
            base_prompt: The original work update prompt

        Returns:
            Enhanced prompt with git context if available
        """
        context = self.format_work_update_context()

        if not context:
            return base_prompt

        # Inject context into prompt
        if "Git Context:" not in base_prompt:
            enhanced = f"{base_prompt}\n\nGit Context:\n{context}"
            return enhanced

        return base_prompt


def enhance_work_update_prompt(base_prompt: str, repo_path: str = ".") -> str:
    """
    Convenience function to enhance a work update prompt.

    Args:
        base_prompt: The original prompt
        repo_path: Path to git repository

    Returns:
        Enhanced prompt with git context
    """
    enhancer = WorkUpdateEnhancer(repo_path=repo_path)
    return enhancer.enhance_prompt(base_prompt)


def get_work_context(repo_path: str = ".") -> Optional[Dict[str, any]]:
    """
    Get complete work context for a repository.

    Returns:
        {
            "branch": str,
            "pr_metadata": Dict,
            "changes": Dict,
            "related_commits": List[Dict]
        }
    """
    enhancer = WorkUpdateEnhancer(repo_path=repo_path)

    return {
        "branch": enhancer.get_branch_context(),
        "changes": enhancer.get_change_context(),
        "related_commits": enhancer.get_related_work()
    }
