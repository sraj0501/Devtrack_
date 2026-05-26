"""
devtrack azure-sync — fetch Azure DevOps work items and report status.

Persists a local snapshot to Data/azure/sync_state.json.

Sync modes:
    devtrack azure-sync              # Full resync (default)
    devtrack azure-sync --full       # Explicit full resync (clears cache)
    devtrack azure-sync --hours 24   # Only items changed in last 24h (merges)

Configure default window via .env:
    AZURE_SYNC_WINDOW_HOURS=24       # 0 = full sync (default)
"""

import argparse
import asyncio
import os
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


async def run(full: bool, hours: int | None) -> int:
    from backend.azure.client import AzureDevOpsClient
    from backend.azure.sync import AzureSync

    client = AzureDevOpsClient()
    if not client.is_configured():
        print("ERROR: Azure DevOps not configured. Run 'devtrack azure-check'.")
        return 1

    mode_label = "Full resync" if (full or hours is None) else f"Last {hours}h"
    print(f"Azure DevOps Sync  [{client._org}/{client._project}]  ({mode_label})")
    print("=" * 55)
    print()

    sync = AzureSync(client=client)

    print("Fetching work items...", flush=True)
    try:
        result = await sync.sync(full=full, hours=hours)

        total = result["total"]
        fetched = result["fetched"]
        by_state = result["by_state"]
        items = result["items"]
        mode = result["mode"]

        if fetched == 0 and total == 0:
            print("No work items assigned to you.")
            print()
            from backend.config import get_azure_sync_states
            states_list = get_azure_sync_states()
            sync_states = ", ".join(states_list) if states_list else "any state"
            print(f"States checked: {sync_states}")
            return 0

        if hours is not None:
            print(f"Fetched: {fetched} updated  |  Cache total: {total}")
        else:
            print(f"Work Items: {total} total")

        print("-" * 55)
        for state, count in sorted(by_state.items()):
            bar = "█" * min(count, 30)
            print(f"  {state:<20} {count:>3}  {bar}")
        print()

        display_items = items[:10] if items else []
        if display_items:
            print(f"{'Updated' if hours else 'Items'} (newest {len(display_items)}):")
            print("-" * 55)
            for wi in display_items:
                tags = f"  [{', '.join(wi.tags)}]" if wi.tags else ""
                print(f"  #{wi.id:<6} {wi.state:<15} {wi.work_item_type:<12} {wi.title[:45]}{tags}")
            if fetched > 10:
                print(f"  ... and {fetched - 10} more")
            print()

        print(f"Mode:     {mode}")
        print(f"Saved to: {result['state_file']}")
        print(f"Synced:   {result['last_sync']}")
        return 0

    except Exception as e:
        print(f"ERROR: {e}")
        return 1

    finally:
        await client.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        prog="devtrack azure-sync",
        description="Sync Azure DevOps work items to local cache.",
        add_help=False,
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--full", action="store_true", help="Full resync, clears existing cache")
    group.add_argument("--hours", type=int, metavar="N", help="Only items changed in last N hours (merges)")

    args, _ = parser.parse_known_args()

    sys.exit(asyncio.run(run(full=args.full, hours=args.hours)))
