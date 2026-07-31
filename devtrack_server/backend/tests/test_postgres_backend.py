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

    # Table objects only register onto `metadata` when their owning module is
    # imported (see engine.py's own docstring) — import every module ported
    # onto engine.py so init_all_tables() actually creates their tables.
    import backend.db.ticket_db          # noqa: F401
    import backend.db.learning_store     # noqa: F401
    import backend.db.project_store      # noqa: F401
    import backend.db.platform_store     # noqa: F401
    import backend.admin.user_manager    # noqa: F401
    import backend.db.report_store       # noqa: F401

    engine_mod.reset_engine()
    os.environ["POSTGRES_URL"] = POSTGRES_URL
    try:
        eng = engine_mod.get_engine()
        engine_mod.init_all_tables()
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


def test_init_all_tables_registers_every_migrated_module(pg_engine):
    """metadata.create_all() must succeed for every Table registered by the
    modules already ported onto engine.py (db/{ticket_db,learning_store,
    project_store,platform_store,report_store}.py, admin/user_manager.py) —
    this is the schema-compatibility check that catches SQLite-only DDL
    assumptions."""
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
            assert result.scalar() is True, f"table {table_name!r} missing after init_all_tables()"


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
