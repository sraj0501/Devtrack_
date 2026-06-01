"""
Central SQLAlchemy engine factory for devtrack_server.

  POSTGRES_URL set   → PostgreSQL  (multi-user / production mode)
  POSTGRES_URL unset → SQLite at database_path()  (local / single-user mode)

All Python-owned store modules import `metadata` from here and register their
Table objects against it.  `init_all_tables()` then creates every registered
table in a single `metadata.create_all()` call — safe to call on every startup.

Go-owned tables (`triggers`, `task_updates`, `work_sessions`) are NEVER touched
by Python in PostgreSQL mode.  Stats and active-session data will come via
Go's HTTP stats endpoint (PG-5) or Redis bridge (R-1/R-2).

Usage in a store module
-----------------------
    from sqlalchemy import Column, Text, Integer
    from backend.db.engine import metadata, get_engine, upsert, init_all_tables

    my_table = Table("my_table", metadata,
        Column("id",   Text, primary_key=True),
        Column("name", Text, nullable=False),
    )

    def save(row: dict) -> None:
        update_cols = {k: v for k, v in row.items() if k != "id"}
        stmt = (
            upsert(my_table)
            .values(**row)
            .on_conflict_do_update(index_elements=["id"], set_=update_cols)
        )
        with get_engine().connect() as conn:
            conn.execute(stmt)
            conn.commit()
"""
from __future__ import annotations

import logging
import threading
from typing import Optional
from urllib.parse import urlparse, urlunparse

from sqlalchemy import MetaData, create_engine, event
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Shared metadata registry
# ---------------------------------------------------------------------------
# Every Table defined in a store module with `metadata` as its second arg is
# automatically included in init_all_tables().  Import order does not matter
# as long as the store module has been imported before init_all_tables() runs.
metadata = MetaData()

# ---------------------------------------------------------------------------
# Engine singleton
# ---------------------------------------------------------------------------
_engine: Optional[Engine] = None
_engine_lock = threading.Lock()


def _redact_url(url: str) -> str:
    """Replace the password in a connection URL with *** for safe logging."""
    try:
        p = urlparse(url)
        if p.password:
            netloc = p.netloc.replace(f":{p.password}@", ":***@")
            return urlunparse(p._replace(netloc=netloc))
    except Exception:
        pass
    return url


def _build_engine() -> Engine:
    from backend.config import database_path, postgres_url

    pg_url = postgres_url()
    if pg_url:
        logger.info("db/engine: PostgreSQL mode (%s)", _redact_url(pg_url))
        try:
            engine = create_engine(
                pg_url,
                pool_size=10,
                max_overflow=20,
                pool_timeout=30,
                pool_recycle=1800,
                echo=False,
            )
        except Exception as exc:
            raise RuntimeError(
                f"Failed to create PostgreSQL engine from POSTGRES_URL: {exc}"
            ) from exc
    else:
        db_path = database_path()
        db_path.parent.mkdir(parents=True, exist_ok=True)
        sqlite_url = f"sqlite:///{db_path}"
        logger.info("db/engine: SQLite mode (%s)", db_path)
        engine = create_engine(
            sqlite_url,
            connect_args={"check_same_thread": False, "timeout": 30},
            echo=False,
        )

        @event.listens_for(engine, "connect")
        def _set_sqlite_pragmas(conn, _record):
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute("PRAGMA foreign_keys=ON")

    return engine


def get_engine() -> Engine:
    """Return the process-wide SQLAlchemy engine (lazy-initialised, thread-safe)."""
    global _engine
    if _engine is None:
        with _engine_lock:
            if _engine is None:
                _engine = _build_engine()
    return _engine


def reset_engine() -> None:
    """Dispose the current engine and clear the singleton.

    Used in tests to switch database backends between test cases.
    After calling this, the next get_engine() call picks up current env vars.
    """
    global _engine
    with _engine_lock:
        if _engine is not None:
            _engine.dispose()
            _engine = None


def init_all_tables() -> None:
    """Create all Python-owned tables that have been registered with `metadata`.

    Idempotent — safe to call on every server startup.  Tables are created with
    CREATE TABLE IF NOT EXISTS semantics via checkfirst=True (SQLAlchemy default).
    """
    engine = get_engine()
    metadata.create_all(engine)
    logger.debug("db/engine: schema initialised (%d tables)", len(metadata.tables))


def upsert(table):
    """Return a dialect-aware Insert that supports .on_conflict_do_update().

    Both the SQLite (3.24+) and PostgreSQL dialect Insert classes expose the
    same .on_conflict_do_update(index_elements, set_) API in SQLAlchemy 2.x.
    Calling this helper picks the right dialect automatically.

    Example
    -------
        stmt = (
            upsert(my_table)
            .values(**row)
            .on_conflict_do_update(
                index_elements=["id"],
                set_={k: v for k, v in row.items() if k != "id"},
            )
        )
        with get_engine().connect() as conn:
            conn.execute(stmt)
            conn.commit()
    """
    dialect = get_engine().dialect.name
    if dialect == "postgresql":
        from sqlalchemy.dialects.postgresql import insert
    else:
        from sqlalchemy.dialects.sqlite import insert
    return insert(table)


def is_postgres() -> bool:
    """Return True when the active backend is PostgreSQL."""
    return get_engine().dialect.name == "postgresql"
