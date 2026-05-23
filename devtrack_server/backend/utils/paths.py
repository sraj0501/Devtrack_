"""
Path resolution utilities for DevTrack backend.

Provides fallback path resolution when backend.config is unavailable
(e.g. when run standalone or before .env is loaded).
All paths resolve under project root - no hardcoded ~/.devtrack.
"""

from pathlib import Path


def project_root_path() -> Path:
    """Find project root by walking up from caller, looking for .env or .git."""
    # Use backend package location
    cur = Path(__file__).resolve().parent.parent
    for _ in range(6):
        if (cur / ".env").exists() or (cur / ".env_sample").exists() or (cur / ".git").exists():
            return cur
        parent = cur.parent
        if parent == cur:
            break
        cur = parent
    return Path(__file__).resolve().parent.parent.parent


def fallback_path(*parts: str) -> str:
    """Resolve path under project root when backend.config is unavailable."""
    return str(project_root_path() / Path(*parts))
