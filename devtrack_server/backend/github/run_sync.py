"""
devtrack github-sync — fetch GitHub issues and report status.

Persists a local snapshot to Data/github/sync_state.json.

Sync modes:
    devtrack github-sync              # Full resync (default)
    devtrack github-sync --full       # Explicit full resync (clears cache)
    devtrack github-sync --hours 24   # Only issues updated in last 24h (merges)

Configure default window via .env:
    GITHUB_SYNC_WINDOW_HOURS=24       # 0 = full sync (default)
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
    from backend.github.client import GitHubClient
    from backend.github.sync import GitHubSync

    client = GitHubClient()
    if not client.is_configured():
        print("ERROR: GitHub not configured. Run 'devtrack github-check'.")
        return 1

    mode_label = "Full resync" if (full or hours is None) else f"Last {hours}h"
    print(f"GitHub Sync  [{client._base_url}]  [{client._owner}/{client._repo}]  ({mode_label})")
    print("=" * 60)
    print()

    sync = GitHubSync(client=client)

    print("Fetching issues...", flush=True)
    try:
        result = await sync.sync(full=full, hours=hours)

        total = result["total"]
        fetched = result["fetched"]
        by_state = result["by_state"]
        items = result["items"]
        mode = result["mode"]

        if fetched == 0 and total == 0:
            print("No issues assigned to you.")
            return 0

        if hours is not None:
            print(f"Fetched: {fetched} updated  |  Cache total: {total}")
        else:
            print(f"Issues: {total} total")

        print("-" * 60)
        for state, count in sorted(by_state.items()):
            bar = "█" * min(count, 30)
            print(f"  {state:<20} {count:>3}  {bar}")
        print()

        display_items = items[:10] if items else []
        if display_items:
            print(f"{'Updated' if hours else 'Issues'} (newest {len(display_items)}):")
            print("-" * 60)
            for issue in display_items:
                labels = f"  [{', '.join(issue.labels)}]" if issue.labels else ""
                milestone = f"  [{issue.milestone}]" if issue.milestone else ""
                print(f"  #{issue.number:<6} {issue.state:<12} {issue.title[:45]}{milestone}{labels}")
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
        prog="devtrack github-sync",
        description="Sync GitHub issues to local cache.",
        add_help=False,
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--full", action="store_true", help="Full resync, clears existing cache")
    group.add_argument("--hours", type=int, metavar="N", help="Only issues updated in last N hours (merges)")

    args, _ = parser.parse_known_args()

    sys.exit(asyncio.run(run(full=args.full, hours=args.hours)))
