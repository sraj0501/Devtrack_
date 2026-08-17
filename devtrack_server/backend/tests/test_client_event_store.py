from __future__ import annotations

from pathlib import Path

import pytest
from sqlalchemy import select

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    from backend.db import client_event_store as store

    monkeypatch.delenv("POSTGRES_URL", raising=False)
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    store._schema_done = False
    yield
    reset_engine()
    store._schema_done = False


def _event(ticket_id: str = "TASK-114") -> dict:
    return {
        "event_id": "triggers:1",
        "table_name": "triggers",
        "source_row_id": "1",
        "revision": 1,
        "payload": {"ticket_id": ticket_id, "processed": False},
        "updated_at": "2026-08-11 10:00:00",
    }


def test_replay_upserts_latest_snapshot(isolated_engine):
    from backend.db.client_event_store import (
        client_events_table,
        persist_client_events,
    )
    from backend.db.engine import get_engine

    assert persist_client_events("client-a", [_event()]) == 1
    updated = _event("TASK-999")
    updated["revision"] = 2
    assert persist_client_events("client-a", [updated]) == 1

    with get_engine().connect() as conn:
        rows = conn.execute(select(client_events_table)).mappings().all()
    assert len(rows) == 1
    assert rows[0]["payload"]["ticket_id"] == "TASK-999"
    assert rows[0]["revision"] == 2

    assert persist_client_events("client-a", [_event("TASK-OLD")]) == 1
    with get_engine().connect() as conn:
        row = conn.execute(select(client_events_table)).mappings().one()
    assert row["payload"]["ticket_id"] == "TASK-999"
    assert row["revision"] == 2


def test_list_rows_preserves_server_owned_attribution(isolated_engine):
    from backend.db.client_event_store import list_client_rows, persist_client_events

    event = _event()
    event["payload"]["_client_id"] = "spoofed"
    persist_client_events("  client-a  ", [event])

    rows = list_client_rows("triggers", client_id="client-a")
    assert len(rows) == 1
    assert rows[0]["_client_id"] == "client-a"
    assert rows[0]["_event_id"] == "triggers:1"
    assert rows[0]["_revision"] == 1


@pytest.mark.parametrize(
    "client_id,events,message",
    [
        ("", [_event()], "client_id"),
        ("client-a", [{**_event(), "table_name": "logs"}], "unsupported"),
        ("client-a", [{**_event(), "payload": "not-an-object"}], "payload"),
        ("client-a", [_event(), _event()], "duplicate"),
    ],
)
def test_invalid_batches_fail_before_persistence(isolated_engine, client_id, events, message):
    from backend.db.client_event_store import persist_client_events

    with pytest.raises(ValueError, match=message):
        persist_client_events(client_id, events)
