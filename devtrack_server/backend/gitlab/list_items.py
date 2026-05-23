"""
List GitLab issues assigned to you.

Run via:
    devtrack gitlab-list [--closed] [--state <state>]
    uv run python backend/gitlab/list_items.py
"""

import asyncio
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from pathlib import Path


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


def parse_args():
    state = "opened"
    show_closed = False
    i = 1
    while i < len(sys.argv):
        arg = sys.argv[i]
        if arg == "--closed":
            show_closed = True
            state = "closed"
        elif arg == "--state" and i + 1 < len(sys.argv):
            state = sys.argv[i + 1].strip()
            i += 1
        i += 1
    return state, show_closed


async def run(state="opened", show_closed=False) -> int:
    from backend.gitlab.client import GitLabClient

    client = GitLabClient()

    if not client.is_configured():
        print("ERROR: GitLab not configured. Run 'devtrack gitlab-check' for details.")
        return 1

    header = f"GitLab Issues  [{client._base_url}]"
    print(header)
    print("=" * len(header))
    print(f"State: {state}")
    print()

    items = await client.get_my_issues(state=state, max_results=50)

    if not items:
        print(f"No issues found with state='{state}'.")
        await client.close()
        return 0

    # Group by milestone
    by_milestone = {}
    for issue in items:
        milestone = issue.milestone_title or "(No milestone)"
        by_milestone.setdefault(milestone, []).append(issue)

    for milestone, group in sorted(by_milestone.items()):
        print(f"{milestone} ({len(group)})")
        print("-" * 40)
        for issue in group:
            labels = f"  [{', '.join(issue.labels)}]" if issue.labels else ""
            print(f"  !{issue.iid:<6}  {issue.state:<8}  {issue.title[:60]}{labels}")
        print()

    print(f"Total: {len(items)} issue(s)")
    await client.close()
    return 0


if __name__ == "__main__":
    state, show_closed = parse_args()
    sys.exit(asyncio.run(run(state=state, show_closed=show_closed)))
