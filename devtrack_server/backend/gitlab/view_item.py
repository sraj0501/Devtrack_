"""
devtrack gitlab-view <project_id> <iid> — show full details for a GitLab issue.

Run via:
    devtrack gitlab-view 42 5
    uv run python backend/gitlab/view_item.py 42 5
"""

import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))


def _load_env() -> None:
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        if (parent / ".env").exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(parent / ".env", override=True)
            except ImportError:
                pass
            return


_load_env()


def _strip_html(text: str) -> str:
    """Remove basic HTML tags for terminal display."""
    import re
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.IGNORECASE)
    text = re.sub(r"<[^>]+>", "", text)
    return text.strip()


async def run(project_id: int, issue_iid: int) -> int:
    from backend.gitlab.client import GitLabClient

    client = GitLabClient()
    if not client.is_configured():
        print("ERROR: GitLab not configured. Run 'devtrack gitlab-check'.")
        return 1

    print(f"Fetching GitLab issue !{issue_iid} in project {project_id}...", flush=True)
    issue = await client.get_issue(project_id, issue_iid)
    if not issue:
        print(f"ERROR: Issue !{issue_iid} not found in project {project_id} or not accessible.")
        await client.close()
        return 1

    print()
    print(f"!{issue.iid}  {issue.title}")
    print("=" * 60)
    print(f"  Type:       Issue")
    print(f"  State:      {issue.state}")
    print(f"  Assigned:   {issue.assignee or '(unassigned)'}")
    print(f"  Project:    {issue.project_id}")
    if issue.milestone_title:
        print(f"  Milestone:  {issue.milestone_title}")
    if issue.labels:
        print(f"  Labels:     {', '.join(issue.labels)}")
    if issue.due_date:
        print(f"  Due:        {issue.due_date}")
    if issue.url:
        print(f"  URL:        {issue.url}")
    print()

    if issue.description:
        clean = _strip_html(issue.description)
        if clean:
            print("Description:")
            print("-" * 60)
            for line in clean.splitlines():
                print(f"  {line}")
            print()

    await client.close()
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: devtrack gitlab-view <project_id> <issue_iid>")
        sys.exit(1)

    try:
        project_id = int(sys.argv[1])
        issue_iid = int(sys.argv[2])
    except ValueError:
        print(f"ERROR: Both project_id and iid must be integers.")
        sys.exit(1)

    sys.exit(asyncio.run(run(project_id, issue_iid)))
