"""
DevTrack Auth Package

Provides lightweight, optional authentication.

Design principles:
  - Login is NEVER required for single-user offline mode
  - All auth state is stored locally first
  - Network calls only happen when user explicitly logs in
  - Graceful degradation: any network failure falls back to local cache

Usage:
    from backend.auth import get_session, is_logged_in, login, logout

    if is_logged_in():
        session = get_session()
        print(session.email)
"""

from backend.auth.session import AuthSession, get_session, is_logged_in, clear_session
from backend.auth.local_auth import load_local_session, save_local_session

__all__ = [
    "AuthSession",
    "get_session",
    "is_logged_in",
    "clear_session",
    "load_local_session",
    "save_local_session",
]
