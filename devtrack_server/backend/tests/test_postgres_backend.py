"""
TASK-113 — Postgres test lane.

Proves backend/db/engine.py's dual-dialect factory (and every store module
registered against its shared `metadata`) actually works against a real
PostgreSQL server, not just SQLite. Before this file, zero tests exercised
Postgres even though POSTGRES_URL has been a supported config value since
db/engine.py was introduced — that gap is exactly how the client/server
split-brain (TASK-112) went unnoticed.

Skipped entirely unless POSTGRES_URL is set. Locally:
    docker compose -f devtrack_server/docker-compose.yml up -d postgres
    POSTGRES_URL=postgresql://devtrack:devtrack@localhost:5432/devtrack \
        uv run pytest backend/tests/test_postgres_backend.py -v

In CI this is wired to a postgres service container (see .github/workflows/ci.yml).
"""
import os
import sqlite3
import uuid

import pytest
from sqlalchemy import text

POSTGRES_URL = os.environ.get("POSTGRES_URL")

pytestmark = pytest.mark.skipif(
    not POSTGRES_URL,
    reason="POSTGRES_URL not set — Postgres test lane needs a live server",
)


@pytest.fixture
def pg_engine():
    """A real Postgres engine, schema created, dropped clean afterward.

    Uses backend.db.engine's actual singleton (reset around the test) rather
    than a standalone create_engine() call, so this exercises the exact same
    get_engine()/upsert()/is_postgres() code path production code runs.
    """
    from backend.db import engine as engine_mod

    from backend.db.migrate import upgrade
    import backend.db.schema  # noqa: F401

    engine_mod.reset_engine()
    os.environ["POSTGRES_URL"] = POSTGRES_URL
    try:
        eng = engine_mod.get_engine()
        upgrade()
        yield eng
        with eng.connect() as conn:
            for table in reversed(engine_mod.metadata.sorted_tables):
                conn.execute(text(f'TRUNCATE TABLE "{table.name}" CASCADE'))
            conn.commit()
    finally:
        engine_mod.reset_engine()


def test_dialect_is_postgres(pg_engine):
    from backend.db.engine import is_postgres
    assert is_postgres() is True
    assert pg_engine.dialect.name == "postgresql"


def test_alembic_registers_every_server_table(pg_engine):
    """The migration head must contain every shared server table."""
    from backend.db import engine as engine_mod
    assert len(engine_mod.metadata.tables) > 0
    with pg_engine.connect() as conn:
        for table_name in engine_mod.metadata.tables:
            result = conn.execute(
                text(
                    "SELECT EXISTS (SELECT 1 FROM information_schema.tables "
                    "WHERE table_name = :name)"
                ),
                {"name": table_name},
            )
            assert result.scalar() is True, f"table {table_name!r} missing at migration head"


def test_alembic_initial_revision_includes_admin_schema(pg_engine):
    from backend.admin.schema import admin_metadata

    with pg_engine.connect() as conn:
        for table_name in admin_metadata.tables:
            result = conn.execute(
                text(
                    "SELECT EXISTS (SELECT 1 FROM information_schema.tables "
                    "WHERE table_name = :name)"
                ),
                {"name": table_name},
            )
            assert result.scalar() is True, f"admin table {table_name!r} missing"


def test_legacy_sqlite_import_is_idempotent(pg_engine, tmp_path):
    from backend.db.sqlite_import import import_sqlite

    source_path = tmp_path / "legacy.db"
    source = sqlite3.connect(source_path)
    source.executescript(
        """
        CREATE TABLE ticket_cache (
            id TEXT PRIMARY KEY, source TEXT NOT NULL, external_id TEXT NOT NULL,
            repo TEXT, title TEXT NOT NULL, description TEXT, status TEXT,
            assignee TEXT, labels TEXT, url TEXT, synced_at TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
        INSERT INTO ticket_cache VALUES
            ('github:115', 'github', '115', 'sraj0501/Devtrack_', 'Migrations',
             '', 'open', '', '[]', '', '2026-08-17', '2026-08-17');
        CREATE TABLE triggers (
            id INTEGER PRIMARY KEY, trigger_type TEXT NOT NULL,
            timestamp TEXT NOT NULL, created_at TEXT NOT NULL
        );
        INSERT INTO triggers VALUES (42, 'commit', '2026-08-17T12:00:00Z', '2026-08-17');
        """
    )
    source.commit()
    source.close()

    first = import_sqlite(
        source_path,
        client_id="migration-test-client",
        target_engine=pg_engine,
    )
    second = import_sqlite(
        source_path,
        client_id="migration-test-client",
        target_engine=pg_engine,
    )

    assert first.inserted["ticket_cache"] == 1
    assert first.inserted["client_events"] == 1
    assert second.total_inserted == 0
    with pg_engine.connect() as conn:
        event = conn.execute(
            text(
                "SELECT event_id, revision, payload->>'trigger_type' AS trigger_type "
                "FROM client_events WHERE client_id = :client_id"
            ),
            {"client_id": "migration-test-client"},
        ).mappings().one()
    assert dict(event) == {
        "event_id": "triggers:42",
        "revision": 0,
        "trigger_type": "commit",
    }


def test_upsert_dialect_switch_roundtrip(pg_engine):
    """upsert() must produce a working ON CONFLICT for the postgresql dialect,
    matching the SQLite path every other test already covers."""
    from backend.db.engine import upsert
    from backend.db.ticket_db import ticket_cache_table

    row_id = str(uuid.uuid4())
    row = {
        "id": row_id,
        "source": "github",
        "external_id": "123",
        "repo": "sraj0501/Devtrack_",
        "title": "original title",
        "description": None,
        "status": "open",
        "assignee": None,
        "labels": None,
        "url": None,
        "synced_at": "2026-07-31T00:00:00Z",
        "created_at": "2026-07-31T00:00:00Z",
    }
    with pg_engine.connect() as conn:
        stmt = upsert(ticket_cache_table).values(**row)
        conn.execute(
            stmt.on_conflict_do_update(
                index_elements=["id"],
                set_={k: v for k, v in row.items() if k != "id"},
            )
        )
        conn.commit()

        updated = dict(row, title="updated title")
        stmt = upsert(ticket_cache_table).values(**updated)
        conn.execute(
            stmt.on_conflict_do_update(
                index_elements=["id"],
                set_={k: v for k, v in updated.items() if k != "id"},
            )
        )
        conn.commit()

        result = conn.execute(
            ticket_cache_table.select().where(ticket_cache_table.c.id == row_id)
        ).fetchone()
    assert result is not None
    assert result.title == "updated title"


def test_voice_status_timestamps_roundtrip(pg_engine):
    """The webhook voice-status readers use the live PostgreSQL engine."""
    from backend.db.engine import upsert
    from backend.db.voice_seed_store import (
        latest_seeded_at,
        voice_seeded_commits_table,
    )
    from backend.db.voice_sync_store import (
        latest_synced_at,
        voice_synced_items_table,
    )

    with pg_engine.begin() as conn:
        conn.execute(
            upsert(voice_seeded_commits_table).values(
                hash="task-141-seed",
                repo_path="/repo",
                seeded_at="2026-08-10 10:00:00",
            )
        )
        conn.execute(
            upsert(voice_synced_items_table).values(
                platform="github",
                item_id="task-141-sync",
                context_type="description",
                synced_at="2026-08-10 11:00:00",
            )
        )

    assert latest_seeded_at(pg_engine) == "2026-08-10 10:00:00"
    assert latest_synced_at(pg_engine) == "2026-08-10 11:00:00"


def test_client_event_replay_is_idempotent(pg_engine):
    from backend.db.client_event_store import client_events_table, persist_client_events

    event = {
        "event_id": "triggers:task-114",
        "table_name": "triggers",
        "source_row_id": "114",
        "revision": 1,
        "payload": {"processed": False},
        "updated_at": "2026-08-11 10:00:00",
    }
    assert persist_client_events("live-pg-client", [event], pg_engine) == 1
    event["payload"] = {"processed": True}
    event["revision"] = 2
    assert persist_client_events("live-pg-client", [event], pg_engine) == 1

    with pg_engine.connect() as conn:
        rows = conn.execute(client_events_table.select()).mappings().all()
    matches = [row for row in rows if row["client_id"] == "live-pg-client"]
    assert len(matches) == 1
    assert matches[0]["payload"]["processed"] is True


def test_pending_actions_queue_roundtrip(pg_engine):
    from backend.queue_gateway import QueueGateway

    gateway = QueueGateway()
    action_id = gateway.stage(
        action_type="post_comment",
        target="TASK-114",
        platform="github",
        workspace="devtrack",
        payload={"comment": "PostgreSQL-backed queue"},
        confidence=0.95,
    )
    assert action_id > 0
    action = gateway.get_action(action_id)
    assert action is not None
    assert action["status"] == "pending"
    assert any(item["id"] == action_id for item in gateway.list_pending())

    gateway.mark_posted(action_id)
    assert gateway.get_action(action_id)["status"] == "posted"


def test_synced_client_events_feed_server_readers(pg_engine):
    from datetime import datetime

    from backend.daily_report_generator import DailyReportGenerator
    from backend.db.client_event_store import persist_client_events
    from backend.email_reporter import EmailReporter
    from backend.work_tracker.session_store import WorkSessionStore

    events = [
        {
            "event_id": "triggers:reader",
            "table_name": "triggers",
            "source_row_id": "201",
            "revision": 1,
            "payload": {
                "id": 201,
                "trigger_type": "commit",
                "ticket_id": "TASK-114",
                "commit_message": "feat: sync client events",
                "commit_hash": "reader-hash",
                "timestamp": "2026-08-11 09:00:00",
            },
            "updated_at": "2026-08-11 09:00:00",
        },
        {
            "event_id": "task_updates:reader",
            "table_name": "task_updates",
            "source_row_id": "202",
            "revision": 1,
            "payload": {
                "id": 202,
                "timestamp": "2026-08-11 10:00:00",
                "project": "DevTrack",
                "ticket_id": "TASK-114",
                "status": "in_progress",
                "update_text": "Built the PostgreSQL sync path",
            },
            "updated_at": "2026-08-11 10:00:00",
        },
        {
            "event_id": "work_sessions:reader",
            "table_name": "work_sessions",
            "source_row_id": "203",
            "revision": 1,
            "payload": {
                "id": 203,
                "started_at": "2026-08-11 08:00:00",
                "ended_at": None,
                "ticket_ref": "TASK-114",
                "commits": "[]",
            },
            "updated_at": "2026-08-11 10:00:00",
        },
    ]
    assert persist_client_events("reader-client", events, pg_engine) == 3

    generator = DailyReportGenerator.__new__(DailyReportGenerator)
    commit_rows = generator._query_commit_rows("2026-08-11")
    assert any(row["commit_hash"] == "reader-hash" for row in commit_rows)

    activities = EmailReporter().get_daily_activities(datetime(2026, 8, 11))
    assert any(activity.ticket_id == "TASK-114" for activity in activities)

    sessions = WorkSessionStore().get_sessions_for_date("2026-08-11")
    assert any(session["id"] == 203 for session in sessions)
    assert WorkSessionStore().get_active_session()["id"] == 203
