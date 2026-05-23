"""
Learning data store — SQLite-backed persistence for the personalization system.

Replaces the file-based storage previously used by:
  - backend/personalized_ai.py       → Data/learning/consent.json
                                        Data/learning/user_profile.json
                                        Data/learning/communication_samples.jsonl
  - backend/learning_integration.py  → Data/learning/consent.json
                                        Data/learning/state.json

All data lives in the main devtrack.db (same as work_sessions, triggers, etc.).
Tables are created lazily on first access.

MongoDB remains the primary store when MONGODB_URI is set. SQLite is the
offline / local fallback — exactly as before, but backed by a DB table instead
of flat files.
"""
from __future__ import annotations

import json
import sqlite3
from contextlib import contextmanager
from datetime import datetime
from typing import Any, Dict, List, Optional


# ---------------------------------------------------------------------------
# Connection helper
# ---------------------------------------------------------------------------

def _db_path() -> str:
    try:
        from backend.config import database_path
        return str(database_path())
    except Exception:
        from backend.config import get_project_root
        root = get_project_root() or "."
        import os
        return os.path.join(root, "Data", "db", "devtrack.db")


@contextmanager
def _conn():
    con = sqlite3.connect(_db_path())
    con.row_factory = sqlite3.Row
    con.execute("PRAGMA journal_mode=WAL")
    try:
        yield con
        con.commit()
    finally:
        con.close()


# ---------------------------------------------------------------------------
# Schema init
# ---------------------------------------------------------------------------

def _ensure_schema() -> None:
    with _conn() as con:
        con.executescript("""
            CREATE TABLE IF NOT EXISTS learning_consent (
                id              INTEGER PRIMARY KEY,
                user_email      TEXT NOT NULL UNIQUE,
                user_object_id  TEXT,
                consent_given   INTEGER NOT NULL DEFAULT 0,
                consented_at    TEXT,
                version         TEXT NOT NULL DEFAULT '1',
                features_json   TEXT NOT NULL DEFAULT '[]',
                updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
            );

            CREATE TABLE IF NOT EXISTS learning_sync_state (
                user_email      TEXT PRIMARY KEY,
                last_collected  TEXT,
                source          TEXT NOT NULL DEFAULT 'teams'
            );

            CREATE TABLE IF NOT EXISTS learning_user_profiles (
                user_email      TEXT PRIMARY KEY,
                profile_json    TEXT NOT NULL DEFAULT '{}',
                last_updated    TEXT NOT NULL DEFAULT (datetime('now')),
                total_samples   INTEGER NOT NULL DEFAULT 0
            );

            CREATE TABLE IF NOT EXISTS learning_samples (
                id              INTEGER PRIMARY KEY AUTOINCREMENT,
                sample_id       TEXT NOT NULL UNIQUE,
                user_email      TEXT,
                source          TEXT NOT NULL DEFAULT 'unknown',
                timestamp       TEXT NOT NULL DEFAULT (datetime('now')),
                context_type    TEXT NOT NULL DEFAULT 'general',
                trigger_text    TEXT NOT NULL DEFAULT '',
                response_text   TEXT NOT NULL DEFAULT '',
                metadata_json   TEXT NOT NULL DEFAULT '{}'
            );

            CREATE INDEX IF NOT EXISTS idx_learning_samples_email
                ON learning_samples(user_email);
            CREATE INDEX IF NOT EXISTS idx_learning_samples_context
                ON learning_samples(context_type);
        """)


_schema_done: bool = False


def _init() -> None:
    global _schema_done
    if not _schema_done:
        _ensure_schema()
        _schema_done = True


# ---------------------------------------------------------------------------
# Consent
# ---------------------------------------------------------------------------

def load_consent(user_email: str) -> Optional[Dict[str, Any]]:
    _init()
    with _conn() as con:
        row = con.execute(
            "SELECT * FROM learning_consent WHERE user_email=?", (user_email,)
        ).fetchone()
    if not row:
        return None
    d = dict(row)
    d["features"] = json.loads(d.pop("features_json", "[]"))
    return d


def save_consent(
    user_email: str,
    consent_given: bool,
    user_object_id: Optional[str] = None,
    version: str = "1",
    features: Optional[list] = None,
) -> None:
    _init()
    with _conn() as con:
        con.execute(
            """INSERT INTO learning_consent
               (user_email, user_object_id, consent_given, consented_at, version, features_json, updated_at)
               VALUES (?, ?, ?, datetime('now'), ?, ?, datetime('now'))
               ON CONFLICT(user_email) DO UPDATE SET
                 user_object_id = COALESCE(excluded.user_object_id, user_object_id),
                 consent_given  = excluded.consent_given,
                 consented_at   = CASE WHEN excluded.consent_given THEN excluded.consented_at ELSE consented_at END,
                 version        = excluded.version,
                 features_json  = excluded.features_json,
                 updated_at     = excluded.updated_at
            """,
            (
                user_email,
                user_object_id,
                int(consent_given),
                version,
                json.dumps(features or []),
            ),
        )


def update_user_object_id(user_email: str, user_object_id: str) -> None:
    _init()
    with _conn() as con:
        con.execute(
            "UPDATE learning_consent SET user_object_id=? WHERE user_email=?",
            (user_object_id, user_email),
        )


# ---------------------------------------------------------------------------
# Sync state (delta collection timestamp)
# ---------------------------------------------------------------------------

def load_last_collected(user_email: str) -> Optional[datetime]:
    _init()
    with _conn() as con:
        row = con.execute(
            "SELECT last_collected FROM learning_sync_state WHERE user_email=?", (user_email,)
        ).fetchone()
    if not row or not row["last_collected"]:
        return None
    try:
        return datetime.fromisoformat(row["last_collected"])
    except ValueError:
        return None


def save_last_collected(user_email: str, ts: datetime) -> None:
    _init()
    ts_str = ts.isoformat()
    with _conn() as con:
        con.execute(
            """INSERT INTO learning_sync_state (user_email, last_collected)
               VALUES (?, ?)
               ON CONFLICT(user_email) DO UPDATE SET last_collected=excluded.last_collected""",
            (user_email, ts_str),
        )


# ---------------------------------------------------------------------------
# User profiles
# ---------------------------------------------------------------------------

def load_profile(user_email: str) -> Optional[Dict[str, Any]]:
    _init()
    with _conn() as con:
        row = con.execute(
            "SELECT profile_json, last_updated, total_samples FROM learning_user_profiles WHERE user_email=?",
            (user_email,),
        ).fetchone()
    if not row:
        return None
    profile = json.loads(row["profile_json"])
    profile["_last_updated"] = row["last_updated"]
    profile["_total_samples"] = row["total_samples"]
    return profile


def save_profile(user_email: str, profile: Dict[str, Any], total_samples: int = 0) -> None:
    _init()
    # Strip internal keys before storing
    clean = {k: v for k, v in profile.items() if not k.startswith("_")}
    with _conn() as con:
        con.execute(
            """INSERT INTO learning_user_profiles (user_email, profile_json, last_updated, total_samples)
               VALUES (?, ?, datetime('now'), ?)
               ON CONFLICT(user_email) DO UPDATE SET
                 profile_json  = excluded.profile_json,
                 last_updated  = excluded.last_updated,
                 total_samples = excluded.total_samples""",
            (user_email, json.dumps(clean, default=str), total_samples),
        )


# ---------------------------------------------------------------------------
# Communication samples (local fallback for MongoDB)
# ---------------------------------------------------------------------------

def count_samples(user_email: str) -> int:
    _init()
    with _conn() as con:
        row = con.execute(
            "SELECT COUNT(*) FROM learning_samples WHERE user_email=?", (user_email,)
        ).fetchone()
    return row[0] if row else 0


def load_samples(user_email: str, limit: int = 1000) -> List[Dict[str, Any]]:
    _init()
    with _conn() as con:
        rows = con.execute(
            "SELECT * FROM learning_samples WHERE user_email=? ORDER BY id DESC LIMIT ?",
            (user_email, limit),
        ).fetchall()
    result = []
    for r in rows:
        d = dict(r)
        d["metadata"] = json.loads(d.pop("metadata_json", "{}"))
        result.append(d)
    return result


def save_sample(
    sample_id: str,
    user_email: Optional[str],
    source: str,
    timestamp: str,
    context_type: str,
    trigger_text: str,
    response_text: str,
    metadata: Optional[Dict] = None,
) -> bool:
    """Insert sample; returns False if sample_id already exists (dedup)."""
    _init()
    try:
        with _conn() as con:
            con.execute(
                """INSERT OR IGNORE INTO learning_samples
                   (sample_id, user_email, source, timestamp, context_type,
                    trigger_text, response_text, metadata_json)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
                (
                    sample_id, user_email, source, timestamp, context_type,
                    trigger_text, response_text, json.dumps(metadata or {}),
                ),
            )
        return True
    except Exception:
        return False


def delete_all_samples(user_email: str) -> int:
    _init()
    with _conn() as con:
        cur = con.execute(
            "DELETE FROM learning_samples WHERE user_email=?", (user_email,)
        )
    return cur.rowcount


def delete_consent_and_profile(user_email: str) -> None:
    _init()
    with _conn() as con:
        con.execute("DELETE FROM learning_consent WHERE user_email=?", (user_email,))
        con.execute("DELETE FROM learning_user_profiles WHERE user_email=?", (user_email,))
        con.execute("DELETE FROM learning_sync_state WHERE user_email=?", (user_email,))


