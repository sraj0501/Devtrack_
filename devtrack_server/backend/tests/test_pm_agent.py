"""
Tests for PMAgent — decomposition, parsing, and creation logic.

All platform API calls are mocked. LLM calls use injected mock providers.
"""
import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from backend.pm_agent import PMAgent, WorkItemNode, DecompositionPlan


# ── Fixtures ─────────────────────────────────────────────────────────────────

SAMPLE_LLM_JSON = """
{
  "items": [
    {"level": 0, "type": "Epic", "title": "Authentication System",
     "description": "Complete auth", "labels": [], "parent_index": null},
    {"level": 1, "type": "User Story", "title": "User Login",
     "description": "Login form", "labels": ["story"], "parent_index": 0},
    {"level": 2, "type": "Task", "title": "Build login form",
     "description": "HTML form", "labels": ["task"], "parent_index": 1},
    {"level": 2, "type": "Bug", "title": "Fix session timeout",
     "description": "Session bug", "labels": ["bug"], "parent_index": 1}
  ]
}
"""


@pytest.fixture
def mock_provider():
    p = MagicMock()
    p.generate.return_value = SAMPLE_LLM_JSON
    return p


@pytest.fixture
def azure_agent(mock_provider):
    return PMAgent(platform="azure", provider=mock_provider)


@pytest.fixture
def gitlab_agent(mock_provider):
    return PMAgent(platform="gitlab", provider=mock_provider)


@pytest.fixture
def github_agent(mock_provider):
    return PMAgent(platform="github", provider=mock_provider)


# ── Decomposition tests ───────────────────────────────────────────────────────

class TestDecomposition:

    def test_decompose_returns_plan(self, azure_agent):
        plan = azure_agent.decompose("Build auth system")
        assert isinstance(plan, DecompositionPlan)
        assert plan.total_count == 4
        assert plan.epic_count == 1
        assert plan.story_count == 1
        assert plan.task_count == 2

    def test_decompose_populates_parent_index(self, azure_agent):
        plan = azure_agent.decompose("Build auth system")
        # item[1] is User Story, parent is item[0] (Epic)
        assert plan.items[1].parent_index == 0
        # item[2] is Task, parent is item[1]
        assert plan.items[2].parent_index == 1

    def test_decompose_calls_llm_once(self, azure_agent, mock_provider):
        azure_agent.decompose("Build auth")
        mock_provider.generate.assert_called_once()

    def test_decompose_injects_platform_types_in_prompt(self, azure_agent, mock_provider):
        azure_agent.decompose("Build auth")
        call_args = mock_provider.generate.call_args
        # Extract prompt from positional or keyword args
        prompt = call_args[0][0] if call_args[0] else call_args[1].get("prompt", "")
        assert "Epic" in prompt
        assert "Task" in prompt or "User Story" in prompt

    def test_decompose_gitlab_uses_milestone_in_prompt(self, gitlab_agent, mock_provider):
        gitlab_agent.decompose("Build auth")
        call_args = mock_provider.generate.call_args
        prompt = call_args[0][0] if call_args[0] else call_args[1].get("prompt", "")
        assert "milestone" in prompt.lower()

    def test_decompose_raises_on_empty_llm_response(self, mock_provider):
        mock_provider.generate.return_value = None
        agent = PMAgent(platform="azure", provider=mock_provider)
        with pytest.raises(ValueError, match="no response"):
            agent.decompose("Anything")

    def test_decompose_raises_on_invalid_json(self, mock_provider):
        mock_provider.generate.return_value = "This is not JSON at all"
        agent = PMAgent(platform="azure", provider=mock_provider)
        with pytest.raises(ValueError, match="invalid JSON"):
            agent.decompose("Anything")

    def test_decompose_handles_json_wrapped_in_prose(self, mock_provider):
        """LLM sometimes returns prose before the JSON block."""
        mock_provider.generate.return_value = (
            "Here is the decomposition:\n" + SAMPLE_LLM_JSON + "\nEnd."
        )
        agent = PMAgent(platform="azure", provider=mock_provider)
        plan = agent.decompose("Anything")
        assert plan.total_count == 4

    def test_max_items_per_level_cap(self, mock_provider):
        """Items beyond max_items_per_level should be truncated."""
        many_tasks = {
            "items": [
                {"level": 0, "type": "Epic", "title": "E", "description": "",
                 "labels": [], "parent_index": None}
            ] + [
                {"level": 1, "type": "Task", "title": f"Task {i}", "description": "",
                 "labels": [], "parent_index": 0}
                for i in range(20)  # 20 level-1 items
            ]
        }
        mock_provider.generate.return_value = json.dumps(many_tasks)
        agent = PMAgent(platform="azure", provider=mock_provider, max_items_per_level=5)
        plan = agent.decompose("Anything")
        level1_count = sum(1 for i in plan.items if i.level == 1)
        assert level1_count <= 5


# ── format_preview tests ──────────────────────────────────────────────────────

class TestFormatPreview:

    def test_preview_contains_all_titles(self, azure_agent):
        plan = azure_agent.decompose("Build auth")
        preview = azure_agent.format_preview(plan)
        assert "Authentication System" in preview
        assert "User Login" in preview
        assert "Build login form" in preview
        assert "Fix session timeout" in preview

    def test_preview_shows_indentation(self, azure_agent):
        plan = azure_agent.decompose("Build auth")
        preview = azure_agent.format_preview(plan)
        lines = preview.splitlines()
        # Epic lines should be less indented than Story lines
        epic_line = next(l for l in lines if "Authentication System" in l)
        story_line = next(l for l in lines if "User Login" in l)
        epic_indent = len(epic_line) - len(epic_line.lstrip())
        story_indent = len(story_line) - len(story_line.lstrip())
        assert story_indent > epic_indent

    def test_preview_shows_item_count_summary(self, azure_agent):
        plan = azure_agent.decompose("Build auth")
        preview = azure_agent.format_preview(plan)
        assert "4" in preview  # total_count


# ── create_all tests (mocked platform clients) ───────────────────────────────

class TestCreateAllAzure:

    @pytest.mark.asyncio
    async def test_create_all_azure_calls_client(self, azure_agent, mock_provider):
        plan = azure_agent.decompose("Build auth")

        mock_client = AsyncMock()
        mock_wi = MagicMock()
        mock_wi.id = 100
        mock_wi.url = "https://dev.azure.com/org/proj/_workitems/100"
        mock_client.create_work_item.return_value = mock_wi
        mock_client.close = AsyncMock()
        azure_agent._azure_client = mock_client

        created, failed = await azure_agent.create_all(plan)
        assert len(failed) == 0
        assert len(created) == 4
        assert mock_client.create_work_item.call_count == 4

    @pytest.mark.asyncio
    async def test_create_all_azure_passes_parent_id(self, azure_agent, mock_provider):
        plan = azure_agent.decompose("Build auth")

        call_args_list = []
        mock_client = AsyncMock()

        async def fake_create(**kwargs):
            call_args_list.append(kwargs)
            wi = MagicMock()
            wi.id = 100 + len(call_args_list)
            wi.url = ""
            return wi

        mock_client.create_work_item.side_effect = fake_create
        mock_client.close = AsyncMock()
        azure_agent._azure_client = mock_client

        await azure_agent.create_all(plan)
        # Child items should have parent_id set
        user_story_call = call_args_list[1]  # index 1 = User Story
        assert user_story_call.get("parent_id") == 101  # Epic was created first as 101

    @pytest.mark.asyncio
    async def test_create_all_partial_failure_continues(self, azure_agent, mock_provider):
        plan = azure_agent.decompose("Build auth")

        call_count = [0]
        mock_client = AsyncMock()

        async def fake_create(**kwargs):
            call_count[0] += 1
            if call_count[0] == 2:  # Fail on second item
                return None
            wi = MagicMock()
            wi.id = 100 + call_count[0]
            wi.url = ""
            return wi

        mock_client.create_work_item.side_effect = fake_create
        mock_client.close = AsyncMock()
        azure_agent._azure_client = mock_client

        created, failed = await azure_agent.create_all(plan)
        assert len(failed) >= 1
        # At least 2 items should be skipped/failed (the failing one + its children)
        assert len(created) < 4


class TestCreateAllGitLab:

    @pytest.mark.asyncio
    async def test_create_all_gitlab_creates_milestone_for_epic(self, gitlab_agent, mock_provider):
        mock_provider.generate.return_value = SAMPLE_LLM_JSON
        plan = gitlab_agent.decompose("Build auth")

        mock_client = AsyncMock()
        mock_client._project_id = 42
        mock_client._base_url = "https://gitlab.com"
        # Milestone creation via _post
        mock_client._post = AsyncMock(return_value={"id": 5, "title": "Authentication System", "web_url": ""})
        # Issue creation
        mock_issue = MagicMock()
        mock_issue.id = 100
        mock_issue.iid = 1
        mock_issue.url = ""
        mock_client.create_issue = AsyncMock(return_value=mock_issue)
        mock_client._api = MagicMock(side_effect=lambda path: f"https://gitlab.com/api/v4/{path}")
        mock_client.close = AsyncMock()
        gitlab_agent._gitlab_client = mock_client

        created, failed = await gitlab_agent.create_all(plan)
        # Epic (level 0) should trigger milestone creation via _post
        mock_client._post.assert_called()
        # Stories/tasks (level 1/2) should trigger issue creation
        mock_client.create_issue.assert_called()


class TestCreateAllGitHub:

    @pytest.mark.asyncio
    async def test_create_all_github_creates_milestone_for_epic(self, github_agent, mock_provider):
        mock_provider.generate.return_value = SAMPLE_LLM_JSON
        plan = github_agent.decompose("Build auth")

        def make_resp(number, url):
            resp = MagicMock()
            resp.status = 201
            resp.json = AsyncMock(return_value={"number": number, "html_url": url})
            resp.text = AsyncMock(return_value="")
            return resp

        def make_ctx(resp):
            """Return an object usable as `async with session.post(...) as resp:`"""
            cm = MagicMock()
            cm.__aenter__ = AsyncMock(return_value=resp)
            cm.__aexit__ = AsyncMock(return_value=False)
            return cm

        responses = [
            make_ctx(make_resp(1, "https://github.com/org/repo/milestone/1")),  # Epic milestone
            make_ctx(make_resp(10, "https://github.com/org/repo/issues/10")),   # Story issue
            make_ctx(make_resp(11, "https://github.com/org/repo/issues/11")),   # Task issue
            make_ctx(make_resp(12, "https://github.com/org/repo/issues/12")),   # Bug issue
        ]

        with patch("aiohttp.ClientSession") as mock_session_cls:
            # session.post() is a regular call (not awaited) that returns a ctx manager
            mock_session = MagicMock()
            mock_session.post.side_effect = responses
            # ClientSession used as async context manager
            mock_session_cls.return_value.__aenter__ = AsyncMock(return_value=mock_session)
            mock_session_cls.return_value.__aexit__ = AsyncMock(return_value=False)

            github_agent._github_token = "fake-token"

            with patch.dict("os.environ", {"GITHUB_OWNER": "org", "GITHUB_REPO": "repo"}):
                created, failed = await github_agent.create_all(plan)

        # At minimum the milestone (level 0) should succeed
        assert any(n.level == 0 for n in created)


# ── Platform validation test ──────────────────────────────────────────────────

def test_invalid_platform_raises():
    with pytest.raises(ValueError, match="Unsupported platform"):
        PMAgent(platform="trello")
