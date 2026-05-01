"""
GitHub Pull Request Analyzer

Lists PRs by author and generates LLM-powered summaries suitable for standups.

Configuration (via .env):
    GITHUB_TOKEN    = ghp_... (required)
    GITHUB_OWNER    = your_org_or_username (optional default owner)
    GITHUB_REPO     = repo_name (optional default repo)
"""

import logging
from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional

logger = logging.getLogger(__name__)


@dataclass
class PRSummary:
    """Represents a GitHub Pull Request with an optional AI summary."""

    number: int
    title: str
    state: str                    # 'open', 'closed', 'merged'
    author: str
    created_at: datetime
    updated_at: datetime
    url: str
    repo: str
    base_branch: str
    head_branch: str
    additions: int = 0
    deletions: int = 0
    changed_files: int = 0
    labels: List[str] = field(default_factory=list)
    ai_summary: str = ""


class PRAnalyzer:
    """Fetch and summarize GitHub PRs using the GitHub API + LLM."""

    def __init__(self, token: Optional[str] = None, provider=None):
        """
        Args:
            token:    GitHub PAT. Reads GITHUB_TOKEN from config if None.
            provider: Optional LLM provider (lazy-initialised if None).
        """
        _token = token
        if _token is None:
            try:
                from backend.config import github_token
                _token = github_token()
            except Exception:
                import os
                _token = os.getenv("GITHUB_TOKEN", "")

        self._token = _token or ""
        self._provider = provider  # None = lazy init on first use

    def _get_provider(self):
        if self._provider is None:
            try:
                from backend.llm import get_provider
                self._provider = get_provider()
            except Exception as e:
                logger.warning(f"LLM provider unavailable: {e}")
        return self._provider

    def _get_github(self):
        """Return an authenticated Github client, or raise if no token."""
        from github import Auth, Github
        if not self._token:
            raise ValueError("GITHUB_TOKEN is required for PR analysis")
        return Github(auth=Auth.Token(self._token))

    def _default_owner_repo(self) -> tuple:
        """Read GITHUB_OWNER and GITHUB_REPO from config."""
        try:
            from backend.config import github_owner, github_repo
            return github_owner(), github_repo()
        except Exception:
            import os
            return os.getenv("GITHUB_OWNER", ""), os.getenv("GITHUB_REPO", "")

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def list_prs_by_author(
        self,
        author: str,
        owner: Optional[str] = None,
        repo_name: Optional[str] = None,
        state: str = "open",
        max_results: int = 20,
    ) -> List[PRSummary]:
        """Return PRs authored by *author* in the given repo (or across the org).

        Args:
            author:     GitHub username whose PRs to fetch.
            owner:      GitHub owner / org. Falls back to GITHUB_OWNER from config.
            repo_name:  Repository name. Falls back to GITHUB_REPO from config.
            state:      'open', 'closed', or 'all'.
            max_results: Maximum number of PRs to return.
        """
        try:
            g = self._get_github()
        except Exception as e:
            logger.warning(f"GitHub authentication failed: {e}")
            return []

        default_owner, default_repo = self._default_owner_repo()
        effective_owner = owner or default_owner
        effective_repo = repo_name or default_repo

        try:
            if effective_owner and effective_repo:
                return self._list_repo_prs(g, effective_owner, effective_repo, author, state, max_results)
            elif effective_owner:
                return self._search_org_prs(g, effective_owner, author, state, max_results)
            else:
                logger.warning("No GITHUB_OWNER or repo configured for PR listing")
                return []
        except Exception as e:
            logger.warning(f"Failed to list PRs for {author}: {e}")
            return []

    def summarize_pr(self, pr: PRSummary) -> str:
        """Generate a one-sentence standup summary for *pr* using the LLM.

        Falls back to the PR title if the LLM is unavailable.
        """
        provider = self._get_provider()
        if not provider:
            return pr.title

        prompt = (
            f"Summarize this GitHub PR in one sentence suitable for a standup update.\n"
            f"PR #{pr.number}: {pr.title}\n"
            f"Branch: {pr.head_branch} → {pr.base_branch}\n"
            f"Changed files: {pr.changed_files} (+{pr.additions} -{pr.deletions})\n"
            f"State: {pr.state}\n"
            f"Keep it under 20 words. Focus on the purpose of the change."
        )

        try:
            from backend.llm.base import LLMOptions
            from backend.config import http_timeout_short
            result = provider.generate(
                prompt=prompt,
                options=LLMOptions(temperature=0.3, max_tokens=60),
                timeout=http_timeout_short(),
            )
            return result.strip() if result else pr.title
        except Exception as e:
            logger.warning(f"LLM summarization failed: {e}")
            return pr.title

    def summarize_author_prs(
        self,
        author: str,
        prs: Optional[List[PRSummary]] = None,
        **list_kwargs,
    ) -> Dict:
        """Fetch (or accept) PRs and return a structured summary dict.

        Returns:
            {
                "author": str,
                "total": int,
                "open": int,
                "merged": int,
                "closed": int,
                "prs": List[PRSummary],    # with ai_summary populated
            }
        """
        if prs is None:
            prs = self.list_prs_by_author(author, **list_kwargs)

        open_count = sum(1 for p in prs if p.state == "open")
        merged_count = sum(1 for p in prs if p.state == "merged")
        closed_count = sum(1 for p in prs if p.state == "closed")

        enriched = []
        for pr in prs:
            if not pr.ai_summary:
                pr.ai_summary = self.summarize_pr(pr)
            enriched.append(pr)

        return {
            "author": author,
            "total": len(prs),
            "open": open_count,
            "merged": merged_count,
            "closed": closed_count,
            "prs": enriched,
        }

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _list_repo_prs(
        self, g, owner: str, repo_name: str, author: str, state: str, max_results: int
    ) -> List[PRSummary]:
        """List PRs from a specific repo, filtered by author."""
        repo = g.get_repo(f"{owner}/{repo_name}")
        pulls = repo.get_pulls(state=state if state != "merged" else "closed")
        results = []
        for pr in pulls:
            if pr.user.login.lower() != author.lower():
                continue
            if state == "merged" and not pr.merged:
                continue
            results.append(self._build_summary(pr, repo_name))
            if len(results) >= max_results:
                break
        return results

    def _search_org_prs(
        self, g, owner: str, author: str, state: str, max_results: int
    ) -> List[PRSummary]:
        """Search for PRs by author within an org (no specific repo)."""
        state_q = state if state in ("open", "closed") else "open"
        query = f"is:pr author:{author} is:{state_q} org:{owner}"
        issues = g.search_issues(query)
        results = []
        for item in issues:
            try:
                pr = item.as_pull_request()
                repo_name = item.repository.name if item.repository else ""
                results.append(self._build_summary(pr, repo_name))
            except Exception:
                continue
            if len(results) >= max_results:
                break
        return results

    @staticmethod
    def _build_summary(pr, repo_name: str) -> PRSummary:
        """Convert a PyGitHub PullRequest object to a PRSummary."""
        state = pr.state
        if pr.merged:
            state = "merged"

        labels = [lbl.name for lbl in (pr.labels or [])]

        return PRSummary(
            number=pr.number,
            title=pr.title or "",
            state=state,
            author=pr.user.login if pr.user else "",
            created_at=pr.created_at or datetime.utcnow(),
            updated_at=pr.updated_at or datetime.utcnow(),
            url=pr.html_url or "",
            repo=repo_name,
            base_branch=pr.base.ref if pr.base else "",
            head_branch=pr.head.ref if pr.head else "",
            additions=pr.additions or 0,
            deletions=pr.deletions or 0,
            changed_files=pr.changed_files or 0,
            labels=labels,
        )
