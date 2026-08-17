"""Idempotent persistence for opt-in Go-client event snapshots.

The Go daemon remains SQLite-only and authoritative for local observation.
When the developer enables server event sync, its SQLite outbox sends latest
row snapshots over ``POST /trigger/client_events``. Replays update the same
``(client_id, event_id)`` row, so a lost acknowledgement cannot duplicate data.
"""
from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Iterable, Mapping, Optional

from sqlalchemy import Column, Integer, JSON, Table, Text, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert


ALLOWED_EVENT_TABLES = frozenset(
    {"triggers", "task_updates", "work_sessions", "pending_actions"}
)

client_events_table = Table(
    "client_events",
    metadata,
    Column("client_id", Text, primary_key=True),
    Column("event_id", Text, primary_key=True),
    Column("table_name", Text, nullable=False),
    Column("source_row_id", Text, nullable=False),
    Column("revision", Integer, nullable=False),
    Column("payload", JSON, nullable=False),
    Column("client_updated_at", Text, nullable=False),
    Column("received_at", Text, nullable=False),
)

_schema_done = False


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if engine is None and _schema_done:
        return eng
    metadata.create_all(eng, tables=[client_events_table])
    if engine is None:
        _schema_done = True
    return eng


def persist_client_events(
    client_id: str,
    events: Iterable[Mapping[str, Any]],
    engine: Optional[Engine] = None,
) -> int:
    """Validate and upsert a complete event batch atomically."""
    if not isinstance(client_id, str) or not client_id.strip():
        raise ValueError("client_id is required")
    client_id = client_id.strip()

    received_at = datetime.now(timezone.utc).isoformat()
    rows: list[dict[str, Any]] = []
    seen: set[str] = set()
    for raw in events:
        if not isinstance(raw, Mapping):
            raise ValueError("each event must be an object")
        event_id = raw.get("event_id")
        table_name = raw.get("table_name")
        source_row_id = raw.get("source_row_id")
        revision = raw.get("revision")
        payload = raw.get("payload")
        updated_at = raw.get("updated_at")
        if not isinstance(event_id, str) or not event_id:
            raise ValueError("event_id is required")
        if event_id in seen:
            raise ValueError(f"duplicate event_id in batch: {event_id}")
        seen.add(event_id)
        if table_name not in ALLOWED_EVENT_TABLES:
            raise ValueError(f"unsupported table_name: {table_name!r}")
        if not isinstance(source_row_id, str) or not source_row_id:
            raise ValueError("source_row_id is required")
        if not isinstance(revision, int) or isinstance(revision, bool) or revision <= 0:
            raise ValueError("revision must be a positive integer")
        if not isinstance(payload, dict):
            raise ValueError("payload must be an object")
        if not isinstance(updated_at, str) or not updated_at:
            raise ValueError("updated_at is required")
        rows.append(
            {
                "client_id": client_id,
                "event_id": event_id,
                "table_name": table_name,
                "source_row_id": source_row_id,
                "revision": revision,
                "payload": payload,
                "client_updated_at": updated_at,
                "received_at": received_at,
            }
        )

    if not rows:
        return 0

    eng = _init(engine)
    with eng.begin() as conn:
        for row in rows:
            stmt = upsert(client_events_table).values(**row)
            conn.execute(
                stmt.on_conflict_do_update(
                    index_elements=["client_id", "event_id"],
                    set_={
                        "table_name": row["table_name"],
                        "source_row_id": row["source_row_id"],
                        "revision": row["revision"],
                        "payload": row["payload"],
                        "client_updated_at": row["client_updated_at"],
                        "received_at": row["received_at"],
                    },
                    where=stmt.excluded.revision > client_events_table.c.revision,
                )
            )
    return len(rows)


def list_client_rows(
    table_name: str,
    client_id: Optional[str] = None,
    engine: Optional[Engine] = None,
) -> list[dict[str, Any]]:
    """Return the latest synced payload for each matching client row."""
    if table_name not in ALLOWED_EVENT_TABLES:
        raise ValueError(f"unsupported table_name: {table_name!r}")
    try:
        eng = _init(engine)
        stmt = select(client_events_table).where(
            client_events_table.c.table_name == table_name
        )
        if client_id:
            stmt = stmt.where(client_events_table.c.client_id == client_id)
        with eng.connect() as conn:
            rows = conn.execute(stmt).mappings().all()
        result = []
        for row in rows:
            payload = dict(row["payload"])
            # Preserve attribution without allowing client payload keys to
            # impersonate server-owned identity metadata.
            payload["_client_id"] = row["client_id"]
            payload["_event_id"] = row["event_id"]
            payload["_revision"] = row["revision"]
            result.append(payload)
        return result
    except Exception:
        return []
