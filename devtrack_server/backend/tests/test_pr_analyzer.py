"""
Tests for the GitHub PR Analyzer.

No real network calls are made — all GitHub API and LLM calls are mocked.
"""

import pytest
from datetime import datetime
from unittest.mock import MagicMock, patch


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_mock_pr(
    number=1,
    title="Add feature X",
    state="open",
    merged=False,
    login="alice",
    base_ref="main",
    head_ref="feature/x",
    additions=50,
    deletions=10,
    changed_files=3,
    labels=None,
    html_url="https://github.com/org/repo/pull/1",
):
    """Return a mock PyGitHub PullRequest-like object."""
    pr = MagicMock()
    pr.number = number
    pr.title = title
    pr.state = state
    pr.merged = merged
    pr.user.login = login
    pr.base.ref = base_ref
    pr.head.ref = head_ref
    pr.additions = additions
    pr.deletions = deletions
    pr.changed_files = changed_files
    pr.labels = [MagicMock(name=lbl) for lbl in (labels or [])]
    pr.html_url = html_url
    pr.created_at = datetime(2026, 3, 1, 10, 0)
    pr.updated_at = datetime(2026, 3, 2, 10, 0)
    return pr


class _MockProvider:
    """Test double: always returns a fixed summary."""

    def __init__(self, response="Fixed auth bug in login flow."):
        self._response = response

    def generate(self, prompt, options=None, timeout=30):
        return self._response


class _FailingProvider:
    """Test double: always returns None."""

    def generate(self, prompt, options=None, timeout=30):
        return None


# ---------------------------------------------------------------------------
# PRSummary dataclass
# ---------------------------------------------------------------------------

class TestPRSummary:
    def test_defaults(self):
        from backend.github.pr_analyzer import PRSummary
        pr = PRSummary(
            number=1,
            title="Test PR",
            state="open",
            author="alice",
            created_at=datetime(2026, 3, 1),
            updated_at=datetime(2026, 3, 2),
            url="https://github.com/org/repo/pull/1",
            repo="repo",
            base_branch="main",
            head_branch="feature/x",
        )
        assert pr.additions == 0
        assert pr.deletions == 0
        assert pr.changed_files == 0
        assert pr.labels == []
        assert pr.ai_summary == ""


# ---------------------------------------------------------------------------
# PRAnalyzer.summarize_pr()
# ---------------------------------------------------------------------------

class TestSummarizePR:
    def _make_pr_summary(self, **kwargs):
        from backend.github.pr_analyzer import PRSummary
        defaults = dict(
            number=1, title="Add feature X", state="open", author="alice",
            created_at=datetime(2026, 3, 1), updated_at=datetime(2026, 3, 2),
            url="https://github.com/org/repo/pull/1", repo="repo",
            base_branch="main", head_branch="feature/x",
            additions=50, deletions=10, changed_files=3,
        )
        defaults.update(kwargs)
        return PRSummary(**defaults)

    def test_returns_ai_summary_when_provider_available(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider("Adds feature X to the API."))
        pr = self._make_pr_summary()
        result = analyzer.summarize_pr(pr)
        assert result == "Adds feature X to the API."

    def test_falls_back_to_title_when_provider_unavailable(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=None)
        analyzer._provider = None  # ensure no lazy init
        # Patch get_provider to raise so no provider can be obtained
        with patch("backend.llm.get_provider", side_effect=Exception("no provider")):
            pr = self._make_pr_summary(title="My PR title")
            result = analyzer.summarize_pr(pr)
        assert result == "My PR title"

    def test_falls_back_to_title_when_llm_returns_none(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_FailingProvider())
        pr = self._make_pr_summary(title="My fallback title")
        result = analyzer.summarize_pr(pr)
        assert result == "My fallback title"

    def test_strips_whitespace_from_ai_response(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider("  Refactored auth module.  "))
        pr = self._make_pr_summary()
        result = analyzer.summarize_pr(pr)
        assert result == "Refactored auth module."


# ---------------------------------------------------------------------------
# PRAnalyzer.list_prs_by_author()
# ---------------------------------------------------------------------------

class TestListPRsByAuthor:
    def test_returns_empty_when_no_token(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="", provider=_MockProvider())
        with patch.dict("os.environ", {"GITHUB_TOKEN": ""}, clear=False):
            result = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo")
        assert result == []

    def test_returns_empty_when_github_auth_fails(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_bad", provider=_MockProvider())
        with patch("github.Github", side_effect=Exception("Bad credentials")):
            result = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo")
        assert result == []

    def test_returns_empty_on_repo_api_error(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())

        mock_g = MagicMock()
        mock_g.get_repo.side_effect = Exception("Not Found")

        with patch("github.Github") as mock_gh_cls, patch("github.Auth.Token"):
            mock_gh_cls.return_value = mock_g
            result = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo")

        assert result == []

    def test_filters_by_author(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())

        pr_alice = _make_mock_pr(number=1, login="alice")
        pr_bob = _make_mock_pr(number=2, login="bob")

        mock_repo = MagicMock()
        mock_repo.get_pulls.return_value = [pr_alice, pr_bob]

        mock_g = MagicMock()
        mock_g.get_repo.return_value = mock_repo

        with patch("github.Github") as mock_gh_cls, patch("github.Auth.Token"):
            mock_gh_cls.return_value = mock_g
            results = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo")

        assert all(r.author == "alice" for r in results)
        assert len(results) == 1

    def test_merged_pr_gets_merged_state(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())

        pr_mock = _make_mock_pr(number=3, login="alice", state="closed", merged=True)

        mock_repo = MagicMock()
        mock_repo.get_pulls.return_value = [pr_mock]

        mock_g = MagicMock()
        mock_g.get_repo.return_value = mock_repo

        with patch("github.Github") as mock_gh_cls, patch("github.Auth.Token"):
            mock_gh_cls.return_value = mock_g
            results = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo", state="merged")

        assert len(results) == 1
        assert results[0].state == "merged"

    def test_respects_max_results(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())

        prs = [_make_mock_pr(number=i, login="alice") for i in range(10)]

        mock_repo = MagicMock()
        mock_repo.get_pulls.return_value = prs

        mock_g = MagicMock()
        mock_g.get_repo.return_value = mock_repo

        with patch("github.Github") as mock_gh_cls, patch("github.Auth.Token"):
            mock_gh_cls.return_value = mock_g
            results = analyzer.list_prs_by_author("alice", owner="org", repo_name="repo", max_results=3)

        assert len(results) == 3

    def test_returns_empty_when_no_owner_configured(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())

        with patch("github.Github"), patch("github.Auth.Token"):
            with patch.object(analyzer, "_default_owner_repo", return_value=("", "")):
                result = analyzer.list_prs_by_author("alice")

        assert result == []


# ---------------------------------------------------------------------------
# PRAnalyzer.summarize_author_prs()
# ---------------------------------------------------------------------------

class TestSummarizeAuthorPRs:
    def _make_pr_summary(self, number=1, state="open"):
        from backend.github.pr_analyzer import PRSummary
        return PRSummary(
            number=number, title=f"PR #{number}", state=state, author="alice",
            created_at=datetime(2026, 3, 1), updated_at=datetime(2026, 3, 2),
            url=f"https://github.com/org/repo/pull/{number}", repo="repo",
            base_branch="main", head_branch=f"feature/{number}",
        )

    def test_returns_correct_structure(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())
        prs = [self._make_pr_summary(1, "open"), self._make_pr_summary(2, "merged")]
        result = analyzer.summarize_author_prs("alice", prs=prs)

        assert result["author"] == "alice"
        assert result["total"] == 2
        assert result["open"] == 1
        assert result["merged"] == 1
        assert result["closed"] == 0
        assert len(result["prs"]) == 2

    def test_ai_summary_populated_on_each_pr(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider("AI summary."))
        prs = [self._make_pr_summary(1)]
        result = analyzer.summarize_author_prs("alice", prs=prs)
        assert result["prs"][0].ai_summary == "AI summary."

    def test_skips_summarization_if_already_present(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider("New summary."))
        pr = self._make_pr_summary(1)
        pr.ai_summary = "Existing summary."
        result = analyzer.summarize_author_prs("alice", prs=[pr])
        # ai_summary already set — should not be overwritten
        assert result["prs"][0].ai_summary == "Existing summary."

    def test_fetches_prs_when_not_provided(self):
        from backend.github.pr_analyzer import PRAnalyzer
        analyzer = PRAnalyzer(token="ghp_test", provider=_MockProvider())
        mock_prs = [self._make_pr_summary(1)]
        with patch.object(analyzer, "list_prs_by_author", return_value=mock_prs) as mock_list:
            result = analyzer.summarize_author_prs("alice", owner="org", repo_name="repo")
        mock_list.assert_called_once()
        assert result["total"] == 1
