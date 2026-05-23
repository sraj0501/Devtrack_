"""
Auth session model and in-memory session state.

The session is loaded from local storage on first access.
It is never fetched from the network automatically — only on explicit login.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Optional

from backend.auth.local_auth import load_local_session, save_local_session, clear_local_session

# Session validity: 90 days (matches TERMS.md guarantee)
SESSION_TTL_DAYS = 90

# In-memory cache — populated on first call to get_session()
_session_cache: Optional["AuthSession"] = None


@dataclass
class AuthSession:
    """Represents an authenticated DevTrack user session."""

    email: str
    user_id: str                        # Opaque server-assigned ID
    display_name: str = ""
    token: str = ""                     # Bearer token (local-only for offline)
    token_expires_at: str = ""          # ISO8601 UTC
    tier: str = "personal"             # personal | team | enterprise
    seat_count: int = 1
    telemetry_enabled: bool = False
    logged_in_at: str = ""
    mode: str = "local"                # local | cloud
    organisation: str = ""

    # ── Computed properties ──────────────────────────────────────────────────

    @property
    def is_expired(self) -> bool:
        if not self.token_expires_at:
            return False
        try:
            exp = datetime.fromisoformat(self.token_expires_at)
            return datetime.now(timezone.utc) > exp
        except ValueError:
            return False

    @property
    def is_valid(self) -> bool:
        return bool(self.email) and bool(self.token) and not self.is_expired

    # ── Serialisation ────────────────────────────────────────────────────────

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> "AuthSession":
        known = {f.name for f in cls.__dataclass_fields__.values()}  # type: ignore[attr-defined]
        return cls(**{k: v for k, v in data.items() if k in known})

    def save(self) -> None:
        save_local_session(self.to_dict())

    def __str__(self) -> str:
        status = "valid" if self.is_valid else ("expired" if self.is_expired else "incomplete")
        return (
            f"AuthSession(email={self.email!r}, tier={self.tier!r}, "
            f"mode={self.mode!r}, status={status})"
        )


# ── Public API ───────────────────────────────────────────────────────────────

def get_session() -> Optional[AuthSession]:
    """
    Return the current session, loading from local storage if needed.
    Returns None if no valid session exists.
    """
    global _session_cache

    if _session_cache is not None:
        return _session_cache if _session_cache.is_valid else None

    data = load_local_session()
    if data is None:
        return None

    try:
        session = AuthSession.from_dict(data)
        if session.is_valid:
            _session_cache = session
            return session
    except Exception:
        pass
    return None


def is_logged_in() -> bool:
    """Return True if there is a valid, non-expired local session."""
    return get_session() is not None


def set_session(session: AuthSession) -> None:
    """Store a session in memory and persist to disk."""
    global _session_cache
    _session_cache = session
    session.save()


def clear_session() -> None:
    """Remove session from memory and disk."""
    global _session_cache
    _session_cache = None
    clear_local_session()


def make_offline_session(email: str, display_name: str = "") -> AuthSession:
    """
    Create a local-only session for offline / personal use.
    No network call is made. Token is a local UUID.
    """
    import uuid
    expires = datetime.now(timezone.utc) + timedelta(days=SESSION_TTL_DAYS)
    return AuthSession(
        email=email,
        user_id=f"local-{uuid.uuid4().hex[:12]}",
        display_name=display_name or email.split("@")[0],
        token=f"local-{uuid.uuid4().hex}",
        token_expires_at=expires.isoformat(),
        tier="personal",
        telemetry_enabled=False,
        logged_in_at=datetime.now(timezone.utc).isoformat(),
        mode="local",
    )
