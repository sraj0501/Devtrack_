"""
DevTrack License Manager

Handles:
  - First-run Terms & Conditions acceptance (local file, no network required)
  - License tier detection (Personal / Team / Enterprise)
  - Acceptance persistence in Data/license/acceptance.json

Design principles:
  - Single-user offline mode ALWAYS works — no network calls here
  - Never blocks execution if acceptance file is unreadable
  - Tier enforcement is advisory in local mode; strict in SaaS mode
"""

from __future__ import annotations

import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from backend.config import _find_project_root

# ── Constants ────────────────────────────────────────────────────────────────

TERMS_VERSION = "1.0"
FREE_TEAM_SEAT_LIMIT = 10

LICENSE_TIERS = {
    "personal": "Personal (Free)",
    "team": "Team (Free, ≤10 users)",
    "enterprise": "Enterprise (Commercial licence required)",
}

_TERMS_SUMMARY = """\
╔══════════════════════════════════════════════════════════════════╗
║              DevTrack — Terms of Service v{version}                 ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  LICENCE TIERS                                                   ║
║  • Personal   — Free. Single user, personal use.                 ║
║  • Team       — Free for up to 10 users. Self-hosted.            ║
║  • Enterprise — Commercial licence required for 11+ users        ║
║                 or hosted/SaaS deployments.                      ║
║                                                                  ║
║  DATA & PRIVACY                                                  ║
║  • Offline/local mode: zero data collection (default).           ║
║  • Telemetry is strictly opt-in — only when you log in           ║
║    and explicitly enable it.                                     ║
║  • Your code, commits, and work updates never leave              ║
║    your machine unless you enable cloud features.                ║
║                                                                  ║
║  OFFLINE GUARANTEE                                               ║
║  • DevTrack always works offline, no licence server needed.      ║
║  • Login is optional for personal use.                           ║
║                                                                  ║
║  Full terms: TERMS.md in the DevTrack installation directory     ║
║  or https://devtrack.dev/terms                                   ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝"""


# ── Paths ────────────────────────────────────────────────────────────────────

def _license_dir() -> Path:
    """Return the Data/license directory, creating it if needed."""
    from backend.config import get_data_dir
    data_dir = get_data_dir()
    if data_dir:
        base = Path(data_dir)
    else:
        base = _find_project_root() / "Data"
    lic_dir = base / "license"
    lic_dir.mkdir(parents=True, exist_ok=True)
    return lic_dir


def _acceptance_path() -> Path:
    return _license_dir() / "acceptance.json"


# ── Acceptance record ────────────────────────────────────────────────────────

def _load_acceptance() -> Optional[dict]:
    """Load acceptance record from disk. Returns None if not found or corrupt."""
    path = _acceptance_path()
    if not path.exists():
        return None
    try:
        with path.open() as f:
            data = json.load(f)
        # Validate required fields
        if data.get("terms_version") and data.get("accepted_at"):
            return data
    except Exception:
        pass
    return None


def _save_acceptance(user_identifier: Optional[str] = None) -> dict:
    """Persist acceptance to disk. Returns the saved record."""
    record = {
        "terms_version": TERMS_VERSION,
        "accepted_at": datetime.now(timezone.utc).isoformat(),
        "user_identifier": user_identifier or _default_user_identifier(),
        "mode": "local",
    }
    path = _acceptance_path()
    with path.open("w") as f:
        json.dump(record, f, indent=2)
    return record


def _default_user_identifier() -> str:
    """Best-effort local user identifier (no network)."""
    return os.getenv("USER") or os.getenv("USERNAME") or "local-user"


# ── Public API ───────────────────────────────────────────────────────────────

def is_accepted() -> bool:
    """Return True if the current terms version has been accepted."""
    record = _load_acceptance()
    if record is None:
        return False
    return record.get("terms_version") == TERMS_VERSION


def get_acceptance_record() -> Optional[dict]:
    """Return the full acceptance record, or None."""
    return _load_acceptance()


def prompt_and_accept(non_interactive: bool = False) -> bool:
    """
    Show the T&C summary and prompt the user to accept.

    Returns True if accepted, False if declined.
    In non_interactive mode (CI/scripts), auto-accepts and logs a warning.
    """
    print()
    print(_TERMS_SUMMARY.format(version=TERMS_VERSION))
    print()

    if non_interactive:
        print("[devtrack] Non-interactive mode detected — auto-accepting terms.")
        print("[devtrack] Set DEVTRACK_AUTO_ACCEPT_TERMS=1 to suppress this message.")
        _save_acceptance()
        return True

    while True:
        try:
            answer = input(
                "Do you accept these Terms of Service? [yes/no]: "
            ).strip().lower()
        except (EOFError, KeyboardInterrupt):
            print("\n[devtrack] Terms not accepted. Exiting.")
            return False

        if answer in ("yes", "y"):
            _save_acceptance()
            print()
            print("[devtrack] Terms accepted. Welcome to DevTrack!")
            print(f"[devtrack] Acceptance recorded at: {_acceptance_path()}")
            print()
            return True
        elif answer in ("no", "n"):
            print()
            print("[devtrack] Terms not accepted. DevTrack will not start.")
            print("[devtrack] You can review the full terms in TERMS.md")
            return False
        else:
            print("Please type 'yes' or 'no'.")


def ensure_accepted(non_interactive: bool = False) -> bool:
    """
    Check acceptance; prompt if not yet accepted.

    Returns True if accepted (or already was), False if declined.
    Never raises — safe to call at startup.
    """
    # Allow CI/automation to skip via env var
    from backend.config import get_devtrack_auto_accept_terms
    if get_devtrack_auto_accept_terms():
        if not is_accepted():
            _save_acceptance()
        return True

    try:
        if is_accepted():
            return True
        return prompt_and_accept(non_interactive=non_interactive)
    except Exception:
        # Never block startup on license check failures
        return True


# ── Tier detection ───────────────────────────────────────────────────────────

def detect_tier(active_user_count: int = 1) -> str:
    """
    Return the licence tier string based on active user count.

      1        → 'personal'
      2–10     → 'team'
      11+      → 'enterprise'
    """
    if active_user_count <= 1:
        return "personal"
    if active_user_count <= FREE_TEAM_SEAT_LIMIT:
        return "team"
    return "enterprise"


def get_tier_label(tier: str) -> str:
    return LICENSE_TIERS.get(tier, tier)


def check_seat_limit(active_user_count: int) -> tuple[bool, str]:
    """
    Check whether the current user count is within free tier limits.

    Returns (within_limit: bool, message: str).
    """
    tier = detect_tier(active_user_count)
    if tier == "enterprise":
        return (
            False,
            f"Your installation has {active_user_count} active users, which exceeds "
            f"the free Team tier limit of {FREE_TEAM_SEAT_LIMIT}. "
            "A commercial licence is required. Contact license@devtrack.dev",
        )
    return True, f"Licence OK — {get_tier_label(tier)} ({active_user_count} user(s))"


# ── CLI entry point ──────────────────────────────────────────────────────────

def show_license_status() -> None:
    """Print current licence status to stdout."""
    record = _load_acceptance()
    if record:
        print(f"Terms version : {record.get('terms_version', '?')}")
        print(f"Accepted at   : {record.get('accepted_at', '?')}")
        print(f"User          : {record.get('user_identifier', '?')}")
        print(f"Mode          : {record.get('mode', 'local')}")
    else:
        print("Terms not yet accepted. Run: devtrack terms --accept")


def show_terms() -> None:
    """Print the T&C summary."""
    print(_TERMS_SUMMARY.format(version=TERMS_VERSION))
    terms_path = _find_project_root() / "TERMS.md"
    if terms_path.exists():
        print(f"\nFull terms: {terms_path}")
    else:
        print("\nFull terms: https://devtrack.dev/terms")


if __name__ == "__main__":
    # Quick CLI: python -m backend.license_manager [status|accept|terms]
    cmd = sys.argv[1] if len(sys.argv) > 1 else "status"
    if cmd == "status":
        show_license_status()
    elif cmd == "accept":
        ensure_accepted()
    elif cmd == "terms":
        show_terms()
    else:
        print(f"Unknown command: {cmd}")
        sys.exit(1)
