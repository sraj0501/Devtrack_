"""
List Azure DevOps work items assigned to you.

Run via:
    devtrack azure-list [--all] [--state <state>]
    uv run python backend/azure/list_items.py
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
    states = None
    show_all = False
    i = 1
    while i < len(sys.argv):
        arg = sys.argv[i]
        if arg == "--all":
            show_all = True
        elif arg == "--state" and i + 1 < len(sys.argv):
            states = [s.strip() for s in sys.argv[i + 1].split(",")]
            i += 1
        i += 1
    return states, show_all


async def run(states=None, show_all=False):
    from backend.azure.client import AzureDevOpsClient

    client = AzureDevOpsClient()

    if not client.is_configured():
        print("ERROR: Azure DevOps not configured. Run 'devtrack azure-check' for details.")
        return 1

    if states is None:
        if show_all:
            states = None  # no state filter
        else:
            from backend.config import get_azure_sync_states
            states = get_azure_sync_states()

    org = client._org
    project = client._project
    header = f"Azure DevOps Work Items  [{org}/{project}]"
    print(header)
    print("=" * len(header))
    if states:
        print(f"States: {', '.join(states)}")
    else:
        print("States: all")
    print()

    items = await client.get_my_work_items(states=states, max_results=50)

    if not items:
        print("No work items found assigned to you.")
        await client.close()
        return 0

    # Group by state
    by_state = {}
    for wi in items:
        by_state.setdefault(wi.state, []).append(wi)

    for state, group in sorted(by_state.items()):
        print(f"{state} ({len(group)})")
        print("-" * 40)
        for wi in group:
            tags = f"  [{', '.join(wi.tags)}]" if wi.tags else ""
            print(f"  #{wi.id:<6}  {wi.work_item_type:<12}  {wi.title[:60]}{tags}")
        print()

    print(f"Total: {len(items)} item(s)")
    await client.close()
    return 0


if __name__ == "__main__":
    states, show_all = parse_args()
    sys.exit(asyncio.run(run(states=states, show_all=show_all)))
