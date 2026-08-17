"""Unit coverage for the TASK-115 legacy SQLite importer."""

import sqlite3

import pytest
from sqlalchemy import create_engine

from backend.db.sqlite_import import _client_event_rows, import_sqlite


def _legacy_db(path):
    connection = sqlite3.connect(path)
    connection.executescript(
        """
        CREATE TABLE triggers (
            id INTEGER PRIMARY KEY,
            trigger_type TEXT NOT NULL,
            timestamp TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
        INSERT INTO triggers VALUES
            (7, 'commit', '2026-08-17T10:00:00Z', '2026-08-17T10:00:01Z');

        CREATE TABLE task_updates (
            id INTEGER PRIMARY KEY,
            ticket_id TEXT NOT NULL,
            timestamp TEXT NOT NULL
        );
        INSERT INTO task_updates VALUES
            (9, 'TASK-115', '2026-08-17T11:00:00Z');
        """
    )
    connection.commit()
    connection.close()


def test_client_rows_become_attributable_revision_zero_events(tmp_path):
    source_path = tmp_path / "legacy.db"
    _legacy_db(source_path)
    source = create_engine(f"sqlite:///{source_path}")
    try:
        events = _client_event_rows(source, "laptop-a", "fallback")
    finally:
        source.dispose()

    assert [(event["event_id"], event["revision"]) for event in events] == [
        ("task_updates:9", 0),
        ("triggers:7", 0),
    ]
    assert all(event["client_id"] == "laptop-a" for event in events)
    assert events[0]["client_updated_at"] == "2026-08-17T11:00:00Z"
    assert events[1]["payload"]["trigger_type"] == "commit"


def test_import_requires_postgres_target(tmp_path):
    source_path = tmp_path / "legacy.db"
    _legacy_db(source_path)
    target = create_engine(f"sqlite:///{tmp_path / 'target.db'}")
    try:
        with pytest.raises(RuntimeError, match="PostgreSQL"):
            import_sqlite(source_path, client_id="laptop-a", target_engine=target)
    finally:
        target.dispose()


def test_import_requires_client_attribution(tmp_path):
    source_path = tmp_path / "legacy.db"
    _legacy_db(source_path)
    target = create_engine(f"sqlite:///{tmp_path / 'target.db'}")
    try:
        with pytest.raises(ValueError, match="client_id"):
            import_sqlite(source_path, client_id=" ", target_engine=target)
    finally:
        target.dispose()
