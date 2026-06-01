"""
Learning data store — SQLAlchemy persistence for the personalization system.

Runs against SQLite (local) or PostgreSQL (multi-user), selected via POSTGRES_URL.
MongoDB remains the primary store when MONGODB_URI is set; this module provides
the SQLite/PostgreSQL fallback for offline / local deployments.

Tables
------
learning_consent        — per-user consent record
learning_sync_state     — delta collection timestamp per user
learning_user_profiles  — computed style profile per user
learning_samples        — communication samples for RAG (local fallback for MongoDB)
"""
from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from sqlalchemy import Boolean, Column, Index, Integer, Table, Text, func, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Table definitions
# ---------------------------------------------------------------------------

learning_consent_table = Table(
    "learning_consent", metadata,
    Column("id",             Integer, primary_key=True, autoincrement=True),
    Column("user_email",     Text, nullable=False, unique=True),
    Column("user_object_id", Text),
    Column("consent_given",  Integer, nullable=False),
    Column("consented_at",   Text),
    Column("version",        Text, nullable=False),
    Column("features_json",  Text, nullable=False),
    Column("updated_at",     Text, nullable=False),
)

learning_sync_state_table = Table(
    "learning_sync_state", metadata,
    Column("user_email",     Text, primary_key=True),
    Column("last_collected", Text),
    Column("source",         Text, nullable=False),
)

learning_user_profiles_table = Table(
    "learning_user_profiles", metadata,
    Column("user_email",    Text, primary_key=True),
    Column("profile_json",  Text, nullable=False),
    Column("last_updated",  Text, nullable=False),
    Column("total_samples", Integer, nullable=False),
)

learning_samples_table = Table(
    "learning_samples", metadata,
    Column("id",            Integer, primary_key=True, autoincrement=True),
    Column("sample_id",     Text, nullable=False, unique=True),
    Column("user_email",    Text),
    Column("source",        Text, nullable=False),
    Column("timestamp",     Text, nullable=False),
    Column("context_type",  Text, nullable=False),
    Column("trigger_text",  Text, nullable=False),
    Column("response_text", Text, nullable=False),
    Column("metadata_json", Text, nullable=False),
    Index("idx_learning_samples_email",   "user_email"),
    Index("idx_learning_samples_context", "context_type"),
)

_schema_done: bool = False
_own_tables = [
    learning_consent_table,
    learning_sync_state_table,
    learning_user_profiles_table,
    learning_samples_table,
]


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if not _schema_done:
        metadata.create_all(eng, tables=_own_tables)
        _schema_done = True
    return eng


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


# ---------------------------------------------------------------------------
# Consent
# ---------------------------------------------------------------------------

def load_consent(user_email: str, engine: Optional[Engine] = None) -> Optional[Dict[str, Any]]:
    eng = _init(engine)
    with eng.connect() as conn:
        row = conn.execute(
            select(learning_consent_table).where(
                learning_consent_table.c.user_email == user_email
            )
        ).mappings().first()
    if not row:
        return None
    d = dict(row)
    d["features"] = json.loads(d.pop("features_json", None) or "[]")
    return d


def save_consent(
    user_email: str,
    consent_given: bool,
    user_object_id: Optional[str] = None,
    version: str = "1",
    features: Optional[list] = None,
    engine: Optional[Engine] = None,
) -> None:
    # Fetch existing row to apply merge logic in Python (simpler than dialect SQL)
    existing = load_consent(user_email, engine)
    now = _now()

    if existing:
        final_obj_id = user_object_id or existing.get("user_object_id")
        final_consented_at = now if consent_given else existing.get("consented_at")
    else:
        final_obj_id = user_object_id
        final_consented_at = now if consent_given else None

    row = {
        "user_email":    user_email,
        "user_object_id": final_obj_id,
        "consent_given": int(consent_given),
        "consented_at":  final_consented_at,
        "version":       version,
        "features_json": json.dumps(features or []),
        "updated_at":    now,
    }
    update_cols = {k: v for k, v in row.items() if k != "user_email"}
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            upsert(learning_consent_table)
            .values(**row)
            .on_conflict_do_update(index_elements=["user_email"], set_=update_cols)
        )


def update_user_object_id(
    user_email: str, user_object_id: str, engine: Optional[Engine] = None
) -> None:
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            learning_consent_table.update()
            .where(learning_consent_table.c.user_email == user_email)
            .values(user_object_id=user_object_id)
        )


# ---------------------------------------------------------------------------
# Sync state
# ---------------------------------------------------------------------------

def load_last_collected(
    user_email: str, engine: Optional[Engine] = None
) -> Optional[datetime]:
    eng = _init(engine)
    with eng.connect() as conn:
        row = conn.execute(
            select(learning_sync_state_table.c.last_collected).where(
                learning_sync_state_table.c.user_email == user_email
            )
        ).first()
    if not row or not row.last_collected:
        return None
    try:
        return datetime.fromisoformat(row.last_collected)
    except ValueError:
        return None


def save_last_collected(
    user_email: str, ts: datetime, engine: Optional[Engine] = None
) -> None:
    ts_str = ts.isoformat()
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            upsert(learning_sync_state_table)
            .values(user_email=user_email, last_collected=ts_str, source="teams")
            .on_conflict_do_update(
                index_elements=["user_email"],
                set_={"last_collected": ts_str},
            )
        )


# ---------------------------------------------------------------------------
# User profiles
# ---------------------------------------------------------------------------

def load_profile(user_email: str, engine: Optional[Engine] = None) -> Optional[Dict[str, Any]]:
    eng = _init(engine)
    with eng.connect() as conn:
        row = conn.execute(
            select(
                learning_user_profiles_table.c.profile_json,
                learning_user_profiles_table.c.last_updated,
                learning_user_profiles_table.c.total_samples,
            ).where(learning_user_profiles_table.c.user_email == user_email)
        ).first()
    if not row:
        return None
    profile = json.loads(row.profile_json)
    profile["_last_updated"] = row.last_updated
    profile["_total_samples"] = row.total_samples
    return profile


def save_profile(
    user_email: str,
    profile: Dict[str, Any],
    total_samples: int = 0,
    engine: Optional[Engine] = None,
) -> None:
    clean = {k: v for k, v in profile.items() if not k.startswith("_")}
    now = _now()
    row = {
        "user_email":    user_email,
        "profile_json":  json.dumps(clean, default=str),
        "last_updated":  now,
        "total_samples": total_samples,
    }
    update_cols = {k: v for k, v in row.items() if k != "user_email"}
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            upsert(learning_user_profiles_table)
            .values(**row)
            .on_conflict_do_update(index_elements=["user_email"], set_=update_cols)
        )


# ---------------------------------------------------------------------------
# Communication samples (local fallback for MongoDB)
# ---------------------------------------------------------------------------

def count_samples(user_email: str, engine: Optional[Engine] = None) -> int:
    eng = _init(engine)
    with eng.connect() as conn:
        result = conn.execute(
            select(func.count()).select_from(learning_samples_table).where(
                learning_samples_table.c.user_email == user_email
            )
        ).scalar()
    return int(result or 0)


def load_samples(
    user_email: str, limit: int = 1000, engine: Optional[Engine] = None
) -> List[Dict[str, Any]]:
    eng = _init(engine)
    with eng.connect() as conn:
        rows = conn.execute(
            select(learning_samples_table)
            .where(learning_samples_table.c.user_email == user_email)
            .order_by(learning_samples_table.c.id.desc())
            .limit(limit)
        ).mappings().all()
    result = []
    for r in rows:
        d = dict(r)
        d["metadata"] = json.loads(d.pop("metadata_json", None) or "{}")
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
    engine: Optional[Engine] = None,
) -> bool:
    """Insert sample; returns False if sample_id already exists (dedup)."""
    row = {
        "sample_id":     sample_id,
        "user_email":    user_email,
        "source":        source,
        "timestamp":     timestamp,
        "context_type":  context_type,
        "trigger_text":  trigger_text,
        "response_text": response_text,
        "metadata_json": json.dumps(metadata or {}),
    }
    eng = _init(engine)
    try:
        with eng.begin() as conn:
            result = conn.execute(
                upsert(learning_samples_table)
                .values(**row)
                .on_conflict_do_nothing()
            )
        return result.rowcount > 0
    except Exception as exc:
        logger.debug("save_sample: error: %s", exc)
        return False


def delete_all_samples(user_email: str, engine: Optional[Engine] = None) -> int:
    eng = _init(engine)
    with eng.begin() as conn:
        result = conn.execute(
            learning_samples_table.delete().where(
                learning_samples_table.c.user_email == user_email
            )
        )
    return result.rowcount


def delete_consent_and_profile(user_email: str, engine: Optional[Engine] = None) -> None:
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            learning_consent_table.delete().where(
                learning_consent_table.c.user_email == user_email
            )
        )
        conn.execute(
            learning_user_profiles_table.delete().where(
                learning_user_profiles_table.c.user_email == user_email
            )
        )
        conn.execute(
            learning_sync_state_table.delete().where(
                learning_sync_state_table.c.user_email == user_email
            )
        )
