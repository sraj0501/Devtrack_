"""
devtrack github-view <issue_number> — show full details for a GitHub issue.

Run via:
    devtrack github-view 42
    uv run python backend/github/view_item.py 42
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


async def run(issue_number: int) -> int:
    from backend.github.client import GitHubClient

    client = GitHubClient()
    if not client.is_configured():
        print("ERROR: GitHub not configured. Run 'devtrack github-check'.")
        return 1

    print(f"Fetching GitHub issue #{issue_number} in {client._owner}/{client._repo}...", flush=True)
    issue = await client.get_issue(issue_number)
    if not issue:
        print(f"ERROR: Issue #{issue_number} not found or not accessible.")
        await client.close()
        return 1

    print()
    print(f"#{issue.number}  {issue.title}")
    print("=" * 60)
    print(f"  Type:       Issue")
    print(f"  State:      {issue.state}")
    if issue.assignees:
        print(f"  Assignees:  {', '.join(issue.assignees)}")
    else:
        print(f"  Assignees:  (unassigned)")
    if issue.labels:
        print(f"  Labels:     {', '.join(issue.labels)}")
    if issue.milestone:
        print(f"  Milestone:  {issue.milestone}")
    if issue.html_url:
        print(f"  URL:        {issue.html_url}")
    print()

    if issue.body:
        clean = _strip_html(issue.body)
        if clean:
            print("Body:")
            print("-" * 60)
            for line in clean.splitlines():
                print(f"  {line}")
            print()

    await client.close()
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: devtrack github-view <issue_number>")
        sys.exit(1)

    try:
        issue_number = int(sys.argv[1])
    except ValueError:
        print(f"ERROR: issue_number must be an integer.")
        sys.exit(1)

    sys.exit(asyncio.run(run(issue_number)))
