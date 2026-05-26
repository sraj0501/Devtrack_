"""
Local session storage — reads/writes Data/license/session.json.

No network calls. Works fully offline.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Optional


def _session_path() -> Path:
    from backend.config import get_data_dir, _find_project_root
    data_dir = get_data_dir()
    if data_dir:
        base = Path(data_dir)
    else:
        base = _find_project_root() / "Data"
    lic_dir = base / "license"
    lic_dir.mkdir(parents=True, exist_ok=True)
    return lic_dir / "session.json"


def load_local_session() -> Optional[dict]:
    """Load session dict from disk. Returns None if missing or corrupt."""
    path = _session_path()
    if not path.exists():
        return None
    try:
        with path.open() as f:
            data = json.load(f)
        if isinstance(data, dict) and data.get("email"):
            return data
    except Exception:
        pass
    return None


def save_local_session(data: dict) -> None:
    """Persist session dict to disk."""
    path = _session_path()
    with path.open("w") as f:
        json.dump(data, f, indent=2)
    # Restrict permissions: owner read/write only
    try:
        path.chmod(0o600)
    except Exception:
        pass


def clear_local_session() -> None:
    """Delete the local session file."""
    path = _session_path()
    if path.exists():
        path.unlink()
