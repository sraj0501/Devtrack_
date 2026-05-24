"""
Cloud authentication — email-based magic link login.

This module is only invoked when the user explicitly runs `devtrack login`.
All operations degrade gracefully if the network is unavailable.

Flow:
  1. User provides email → POST /auth/magic-link
  2. Server emails a one-time code
  3. User enters code → POST /auth/verify → receives JWT token
  4. Token + session saved locally via local_auth.save_local_session()

Offline guarantee:
  - If DEVTRACK_API_URL is not set, falls back to local-only session
  - If network is unreachable, falls back to local-only session
  - Existing local sessions always work without network
"""

from __future__ import annotations

import os
import sys
from datetime import datetime, timezone, timedelta
from typing import Optional, Tuple

from backend.auth.session import AuthSession, make_offline_session, set_session, SESSION_TTL_DAYS


# ── Config ───────────────────────────────────────────────────────────────────

def _api_url() -> Optional[str]:
    """Return the DevTrack cloud API URL, or None for offline-only mode."""
    from backend.config import get_devtrack_api_url
    return get_devtrack_api_url() or None


def _is_cloud_available() -> bool:
    return bool(_api_url())


# ── Magic link flow ───────────────────────────────────────────────────────────

def request_magic_link(email: str) -> Tuple[bool, str]:
    """
    Request a magic-link login code for the given email.

    Returns (success: bool, message: str).
    Falls back to local session if cloud unavailable.
    """
    if not _is_cloud_available():
        return False, "Cloud auth not configured (DEVTRACK_API_URL not set)"

    try:
        import httpx
        resp = httpx.post(
            f"{_api_url()}/auth/magic-link",
            json={"email": email},
            timeout=10,
        )
        if resp.status_code == 200:
            return True, f"Login code sent to {email}. Check your inbox."
        return False, f"Server error: {resp.status_code} — {resp.text[:200]}"
    except ImportError:
        return False, "httpx not installed. Run: uv add httpx"
    except Exception as e:
        return False, f"Network error: {e}"


def verify_magic_link(email: str, code: str) -> Tuple[bool, str, Optional[AuthSession]]:
    """
    Verify a one-time code and exchange it for a session token.

    Returns (success: bool, message: str, session: Optional[AuthSession]).
    """
    if not _is_cloud_available():
        return False, "Cloud auth not configured", None

    try:
        import httpx
        resp = httpx.post(
            f"{_api_url()}/auth/verify",
            json={"email": email, "code": code},
            timeout=10,
        )
        if resp.status_code != 200:
            return False, f"Invalid code or expired ({resp.status_code})", None

        data = resp.json()
        expires = datetime.now(timezone.utc) + timedelta(days=SESSION_TTL_DAYS)
        session = AuthSession(
            email=email,
            user_id=data.get("user_id", ""),
            display_name=data.get("display_name", email.split("@")[0]),
            token=data.get("token", ""),
            token_expires_at=data.get("expires_at", expires.isoformat()),
            tier=data.get("tier", "personal"),
            seat_count=data.get("seat_count", 1),
            telemetry_enabled=data.get("telemetry_enabled", False),
            logged_in_at=datetime.now(timezone.utc).isoformat(),
            mode="cloud",
            organisation=data.get("organisation", ""),
        )
        set_session(session)
        return True, "Login successful.", session

    except ImportError:
        return False, "httpx not installed. Run: uv add httpx", None
    except Exception as e:
        return False, f"Network error: {e}", None


# ── Interactive login flow ────────────────────────────────────────────────────

def interactive_login() -> Optional[AuthSession]:
    """
    Full interactive login flow for the terminal.

    Returns the authenticated session, or None if login failed/cancelled.
    If cloud is unavailable, offers local-only session instead.
    """
    print()
    print("DevTrack Login")
    print("─" * 40)

    if not _is_cloud_available():
        print("Cloud login is not configured (DEVTRACK_API_URL not set).")
        print("You can create a local session for offline use.")
        print()
        return _local_only_login()

    try:
        email = input("Email address: ").strip()
    except (EOFError, KeyboardInterrupt):
        print("\nLogin cancelled.")
        return None

    if not email or "@" not in email:
        print("Invalid email address.")
        return None

    print(f"\nSending login code to {email}...")
    ok, msg = request_magic_link(email)
    if not ok:
        print(f"Failed: {msg}")
        print("\nFalling back to local-only session...")
        return _local_only_login(email)

    print(msg)

    try:
        code = input("Enter the 6-digit code from your email: ").strip()
    except (EOFError, KeyboardInterrupt):
        print("\nLogin cancelled.")
        return None

    ok, msg, session = verify_magic_link(email, code)
    if ok and session:
        print(f"\n✓ {msg}")
        print(f"  Logged in as: {session.display_name} ({session.email})")
        print(f"  Tier:         {session.tier}")
        _prompt_telemetry(session)
        return session
    else:
        print(f"\nLogin failed: {msg}")
        return None


def _local_only_login(email: str = "") -> Optional[AuthSession]:
    """Create a local-only (offline) session."""
    if not email:
        try:
            email = input("Enter your email (used as local identifier): ").strip()
        except (EOFError, KeyboardInterrupt):
            return None

    if not email:
        return None

    session = make_offline_session(email)
    set_session(session)
    print(f"\n✓ Local session created for {email}")
    print("  This session works fully offline.")
    print("  To enable cloud features, set DEVTRACK_API_URL and run 'devtrack login'.")
    return session


def _prompt_telemetry(session: AuthSession) -> None:
    """Optionally enable telemetry after login."""
    print()
    print("Telemetry is opt-in and helps improve DevTrack.")
    print("We collect only command usage counts — never your code or data.")
    try:
        answer = input("Enable telemetry? [y/N]: ").strip().lower()
    except (EOFError, KeyboardInterrupt):
        return

    if answer in ("y", "yes"):
        session.telemetry_enabled = True
        session.save()
        print("✓ Telemetry enabled. Thank you!")
    else:
        print("Telemetry remains disabled.")
