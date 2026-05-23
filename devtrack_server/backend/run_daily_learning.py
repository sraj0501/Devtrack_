"""
Daily learning collection script — runs unattended after first-time setup.

Configuration (all via .env):
    LEARNING_CRON_ENABLED=true          Enable cron management
    LEARNING_CRON_SCHEDULE=0 8 * * *    When to run (standard cron expression)
    LEARNING_HISTORY_DAYS=30            Days of history on first run

Usage:
    # Collect now (delta mode — only fetches since last run):
    uv run python backend/run_daily_learning.py

    # Force full re-collection ignoring delta state:
    uv run python backend/run_daily_learning.py --full

    # Install/update crontab entry from LEARNING_CRON_SCHEDULE in .env:
    uv run python backend/run_daily_learning.py --setup-cron

    # Remove the crontab entry:
    uv run python backend/run_daily_learning.py --remove-cron

    # Show current cron status:
    uv run python backend/run_daily_learning.py --cron-status
"""

import asyncio
import argparse
import os
import shutil
import subprocess
import sys
from configparser import ConfigParser
from pathlib import Path

# Allow running from repo root or from backend/
_HERE = Path(__file__).resolve().parent
_ROOT = _HERE.parent
sys.path.insert(0, str(_HERE))
sys.path.insert(0, str(_ROOT))
sys.path.insert(0, str(_HERE / 'msgraph_python'))

# Load .env before importing config functions
try:
    from dotenv import load_dotenv
    for _candidate in [_ROOT / '.env', _ROOT / '.env_sample']:
        if _candidate.exists():
            load_dotenv(_candidate)
            break
except ImportError:
    pass

from backend.config import (
    learning_cron_enabled,
    learning_cron_schedule,
    learning_history_days,
    log_dir,
)
from msgraph_python.graph import Graph
from learning_integration import LearningIntegration

# Marker comment used to identify our crontab entry
_CRON_MARKER = '# devtrack-learning'


def _cron_command() -> str:
    """Build the shell command that goes into the crontab."""
    python = shutil.which('uv') or sys.executable
    script = str(_HERE / 'run_daily_learning.py')
    log = str(log_dir() / 'learning.log')
    if shutil.which('uv'):
        cmd = f'cd {_ROOT} && uv run python {script}'
    else:
        cmd = f'cd {_ROOT} && {python} {script}'
    return f'{cmd} >> {log} 2>&1'


def _read_crontab() -> list[str]:
    """Return current crontab lines (empty list if none)."""
    result = subprocess.run(['crontab', '-l'], capture_output=True, text=True)
    if result.returncode != 0:
        return []
    return result.stdout.splitlines()


def _write_crontab(lines: list[str]) -> None:
    """Write lines as the new crontab."""
    content = '\n'.join(lines) + '\n'
    proc = subprocess.run(['crontab', '-'], input=content, text=True, capture_output=True)
    if proc.returncode != 0:
        print(f"❌ Failed to write crontab: {proc.stderr}")
        sys.exit(1)


def setup_cron() -> None:
    """Install or update the crontab entry using settings from .env."""
    if not learning_cron_enabled():
        print("⚠️  LEARNING_CRON_ENABLED is not set to true in .env.")
        print("   Set LEARNING_CRON_ENABLED=true then re-run --setup-cron.")
        sys.exit(1)

    schedule = learning_cron_schedule()
    cmd = _cron_command()
    new_entry = f'{schedule} {cmd} {_CRON_MARKER}'

    lines = _read_crontab()
    # Remove any existing devtrack-learning entry
    lines = [l for l in lines if _CRON_MARKER not in l]
    lines.append(new_entry)
    _write_crontab(lines)

    print(f"✅ Cron entry installed:")
    print(f"   {new_entry}")
    print()
    print(f"   Schedule : {schedule}")
    print(f"   Log      : {log_dir() / 'learning.log'}")
    print()
    print("   To change the schedule, update LEARNING_CRON_SCHEDULE in .env")
    print("   then re-run: uv run python backend/run_daily_learning.py --setup-cron")


def remove_cron() -> None:
    """Remove the DevTrack learning crontab entry."""
    lines = _read_crontab()
    before = len(lines)
    lines = [l for l in lines if _CRON_MARKER not in l]
    if len(lines) == before:
        print("ℹ️  No DevTrack learning cron entry found.")
        return
    _write_crontab(lines)
    print("✅ Cron entry removed.")


def cron_status() -> None:
    """Show whether the cron entry is installed and what schedule it uses."""
    lines = _read_crontab()
    entries = [l for l in lines if _CRON_MARKER in l]
    if entries:
        print("✅ DevTrack learning cron is ACTIVE:")
        for e in entries:
            print(f"   {e}")
    else:
        print("❌ DevTrack learning cron is NOT installed.")
        print("   Run: uv run python backend/run_daily_learning.py --setup-cron")
    print()
    print(f"   .env settings:")
    print(f"   LEARNING_CRON_ENABLED  = {learning_cron_enabled()}")
    print(f"   LEARNING_CRON_SCHEDULE = {learning_cron_schedule()}")
    print(f"   LEARNING_HISTORY_DAYS  = {learning_history_days()}")


async def reset_learning() -> None:
    """Delete all learning data (MongoDB + files) and re-grant consent."""
    print("⚠️  This will permanently delete ALL learning data.")
    print("   • MongoDB: communication_samples, user_profiles, learning_state")
    print("   • Files:   communication_samples.jsonl, user_profile.json, state.json, consent.json")
    print()
    confirm = input("Type 'yes' to confirm reset: ").strip().lower()
    if confirm != 'yes':
        print("Cancelled.")
        return

    # ── MongoDB ──────────────────────────────────────────────────────────
    try:
        from backend.db.mongo_learning import get_store
        store = get_store()
        if store.is_available():
            # Need user email — load from consent file if present
            import json as _json
            from pathlib import Path as _Path
            from backend.config import learning_dir
            consent_path = _Path(learning_dir()) / "consent.json"
            user_email = ""
            if consent_path.exists():
                try:
                    user_email = _json.loads(consent_path.read_text()).get("user_email", "")
                except Exception:
                    pass
            if user_email:
                deleted = await store.reset_user_data(user_email)
                print(f"✅ MongoDB cleared: {deleted.get('samples', 0)} samples, "
                      f"{deleted.get('profiles', 0)} profiles, "
                      f"{deleted.get('state', 0)} state docs")
            else:
                print("ℹ️  No user email found — skipping MongoDB reset (no consent on file)")
        else:
            print("ℹ️  MongoDB not configured — skipping MongoDB reset")
    except Exception as e:
        print(f"⚠️  MongoDB reset failed: {e}")

    # ── Local files ───────────────────────────────────────────────────────
    try:
        from backend.config import learning_dir
        from pathlib import Path as _Path
        ld = _Path(learning_dir())
        files_deleted = []
        for fname in ("communication_samples.jsonl", "user_profile.json",
                      "state.json", "consent.json"):
            fp = ld / fname
            if fp.exists():
                fp.unlink()
                files_deleted.append(fname)
        if files_deleted:
            print(f"✅ Files deleted: {', '.join(files_deleted)}")
        else:
            print("ℹ️  No local learning files found")
    except Exception as e:
        print(f"⚠️  File cleanup failed: {e}")

    print()
    print("Reset complete. Next steps:")
    print("  1. devtrack enable-learning   — re-grant consent and collect data")
    print("  2. devtrack learning-setup-cron  — install daily cron")


async def run(force_full: bool) -> None:
    """Run a learning collection pass."""
    config_path = _HERE / 'msgraph_python' / 'config.cfg'
    if not config_path.exists():
        print(f"❌ Config not found: {config_path}")
        print("   Set up MS Graph first (see docs/INSTALLATION.md).")
        sys.exit(1)

    config = ConfigParser()
    config.read(config_path)

    print("🔐 Authenticating with Microsoft Graph...")
    graph = Graph(config['azure'])

    integration = LearningIntegration()
    await integration.initialize(graph)

    if not integration.ai or not integration.ai.consent_given:
        print("⚠️  Consent not given. Run 'devtrack enable-learning' first.")
        sys.exit(1)

    days = learning_history_days()
    count = await integration.collect_teams_data(days=days, force_full=force_full)
    integration.show_profile()
    print(f"\nDone. {count} new samples added.")


def main() -> None:
    parser = argparse.ArgumentParser(description="DevTrack daily Teams learning collection")
    group = parser.add_mutually_exclusive_group()
    group.add_argument('--reset', action='store_true',
                       help='Delete all learning data (MongoDB + files) and start fresh')
    group.add_argument('--setup-cron', action='store_true',
                       help='Install/update crontab entry from LEARNING_CRON_SCHEDULE in .env')
    group.add_argument('--remove-cron', action='store_true',
                       help='Remove the DevTrack learning crontab entry')
    group.add_argument('--cron-status', action='store_true',
                       help='Show current cron entry status')
    parser.add_argument('--full', action='store_true',
                        help='Ignore delta state; re-collect LEARNING_HISTORY_DAYS days')
    args = parser.parse_args()

    if args.reset:
        asyncio.run(reset_learning())
    elif args.setup_cron:
        setup_cron()
    elif args.remove_cron:
        remove_cron()
    elif args.cron_status:
        cron_status()
    else:
        asyncio.run(run(force_full=args.full))


if __name__ == '__main__':
    main()
