"""
devtrack azure-view <id> — show full details for a work item.

Run via:
    devtrack azure-view 123
    uv run python backend/azure/view_item.py 123
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


async def run(item_id: int) -> int:
    from backend.azure.client import AzureDevOpsClient

    client = AzureDevOpsClient()
    if not client.is_configured():
        print("ERROR: Azure DevOps not configured. Run 'devtrack azure-check'.")
        return 1

    print(f"Fetching work item #{item_id}...", flush=True)
    wi = await client.get_work_item(item_id)
    if not wi:
        print(f"ERROR: Work item #{item_id} not found or not accessible.")
        await client.close()
        return 1

    print()
    print(f"#{wi.id}  {wi.title}")
    print("=" * 60)
    print(f"  Type:       {wi.work_item_type}")
    print(f"  State:      {wi.state}")
    print(f"  Assigned:   {wi.assigned_to or '(unassigned)'}")
    print(f"  Area:       {wi.area_path}")
    print(f"  Iteration:  {wi.iteration_path or '(none)'}")
    if wi.tags:
        print(f"  Tags:       {', '.join(wi.tags)}")
    if wi.parent_id:
        print(f"  Parent:     #{wi.parent_id}")
    if wi.url:
        print(f"  URL:        {wi.url}")
    print()

    if wi.description:
        clean = _strip_html(wi.description)
        if clean:
            print("Description:")
            print("-" * 60)
            for line in clean.splitlines():
                print(f"  {line}")
            print()

    await client.close()
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: devtrack azure-view <work-item-id>")
        sys.exit(1)

    try:
        item_id = int(sys.argv[1])
    except ValueError:
        print(f"ERROR: '{sys.argv[1]}' is not a valid work item ID.")
        sys.exit(1)

    sys.exit(asyncio.run(run(item_id)))
