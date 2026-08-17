"""One-shot, replay-safe import from legacy SQLite into PostgreSQL.

Server-owned tables are copied to their matching PostgreSQL tables with
``ON CONFLICT DO NOTHING`` so rerunning an interrupted import is safe.  The
four Go-owned activity tables are converted to revision-zero ``client_events``
snapshots.  A later live TASK-114 sync starts at revision one and therefore
supersedes the imported baseline cleanly.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Sequence

from sqlalchemy import Engine, Integer, MetaData, Table, create_engine, inspect, select, text
from sqlalchemy.engine import URL
from sqlalchemy.dialects.postgresql import insert as pg_insert

from backend.admin.schema import admin_metadata
from backend.db.client_event_store import ALLOWED_EVENT_TABLES, client_events_table
from backend.db.engine import get_engine, metadata
from backend.db.schema import sorted_tables  # noqa: F401 - ensures registration


CLIENT_TIMESTAMP_COLUMNS = (
    "updated_at",
    "timestamp",
    "started_at",
    "created_at",
)


@dataclass
class ImportSummary:
    inserted: dict[str, int] = field(default_factory=dict)
    discovered: dict[str, int] = field(default_factory=dict)

    @property
    def total_inserted(self) -> int:
        return sum(self.inserted.values())


def _sqlite_engine(path: Path) -> Engine:
    resolved = path.expanduser().resolve()
    if not resolved.is_file():
        raise FileNotFoundError(f"SQLite database not found: {resolved}")
    return create_engine(URL.create("sqlite", database=str(resolved)))


def _source_tables(engine: Engine) -> set[str]:
    return set(inspect(engine).get_table_names())


def _read_rows(engine: Engine, table_name: str) -> list[dict]:
    source_metadata = MetaData()
    source_table = Table(table_name, source_metadata, autoload_with=engine)
    with engine.connect() as connection:
        return [dict(row) for row in connection.execute(select(source_table)).mappings()]


def _copy_matching_tables(
    source: Engine,
    target_connection,
    target_tables: Iterable[Table],
    summary: ImportSummary,
) -> None:
    available = _source_tables(source)
    for target_table in target_tables:
        if target_table.name not in available:
            continue
        rows = _read_rows(source, target_table.name)
        summary.discovered[target_table.name] = len(rows)
        if not rows:
            summary.inserted[target_table.name] = 0
            continue
        column_names = set(target_table.c.keys())
        compatible_rows = [
            {key: value for key, value in row.items() if key in column_names}
            for row in rows
        ]
        result = target_connection.execute(
            pg_insert(target_table).values(compatible_rows).on_conflict_do_nothing()
        )
        summary.inserted[target_table.name] = max(result.rowcount or 0, 0)


def _client_updated_at(row: dict, fallback: str) -> str:
    for column in CLIENT_TIMESTAMP_COLUMNS:
        value = row.get(column)
        if value is not None and str(value):
            return str(value)
    return fallback


def _client_event_rows(source: Engine, client_id: str, received_at: str) -> list[dict]:
    available = _source_tables(source)
    events: list[dict] = []
    for table_name in sorted(ALLOWED_EVENT_TABLES):
        if table_name not in available:
            continue
        for row in _read_rows(source, table_name):
            source_id = row.get("id")
            if source_id is None:
                continue
            source_row_id = str(source_id)
            events.append(
                {
                    "client_id": client_id,
                    "event_id": f"{table_name}:{source_row_id}",
                    "table_name": table_name,
                    "source_row_id": source_row_id,
                    # Zero is reserved for imported history. Live client sync
                    # starts at one and wins the normal monotonic upsert.
                    "revision": 0,
                    "payload": json.loads(json.dumps(row, default=str)),
                    "client_updated_at": _client_updated_at(row, received_at),
                    "received_at": received_at,
                }
            )
    return events


def _copy_client_events(
    source: Engine,
    target_connection,
    client_id: str,
    summary: ImportSummary,
) -> None:
    received_at = datetime.now(timezone.utc).isoformat()
    rows = _client_event_rows(source, client_id, received_at)
    summary.discovered["client_events"] = len(rows)
    if not rows:
        summary.inserted["client_events"] = 0
        return
    result = target_connection.execute(
        pg_insert(client_events_table).values(rows).on_conflict_do_nothing()
    )
    summary.inserted["client_events"] = max(result.rowcount or 0, 0)


def _reset_integer_sequences(connection, tables: Iterable[Table]) -> None:
    preparer = connection.dialect.identifier_preparer
    for table in tables:
        primary_keys = list(table.primary_key.columns)
        if (
            len(primary_keys) != 1
            or not primary_keys[0].autoincrement
            or not isinstance(primary_keys[0].type, Integer)
        ):
            continue
        column = primary_keys[0]
        table_name = preparer.quote(table.name)
        column_name = preparer.quote(column.name)
        connection.execute(
            text(
                "SELECT setval(pg_get_serial_sequence(:table_name, :column_name), "
                f"GREATEST(COALESCE(MAX({column_name}), 1), 1), "
                f"COUNT({column_name}) > 0) FROM {table_name}"
            ),
            {"table_name": table.name, "column_name": column.name},
        )


def import_sqlite(
    source_path: Path,
    *,
    client_id: str,
    target_engine: Engine | None = None,
    admin_source_path: Path | None = None,
    dry_run: bool = False,
) -> ImportSummary:
    """Import a legacy main database and optional separate ``admin.db``."""
    normalized_client_id = client_id.strip()
    if not normalized_client_id:
        raise ValueError("client_id is required for attributable activity history")
    target = target_engine or get_engine()
    if target.dialect.name != "postgresql":
        raise RuntimeError("SQLite import requires a PostgreSQL POSTGRES_URL")

    source = _sqlite_engine(source_path)
    admin_source = _sqlite_engine(admin_source_path) if admin_source_path else None
    summary = ImportSummary()
    server_tables = [
        table
        for table in metadata.sorted_tables
        if table.name not in {"client_events", "pending_actions"}
    ]

    try:
        if dry_run:
            available = _source_tables(source)
            for table in server_tables:
                if table.name in available:
                    summary.discovered[table.name] = len(_read_rows(source, table.name))
            summary.discovered["client_events"] = len(
                _client_event_rows(
                    source,
                    normalized_client_id,
                    datetime.now(timezone.utc).isoformat(),
                )
            )
            if admin_source:
                available_admin = _source_tables(admin_source)
                for table in admin_metadata.sorted_tables:
                    if table.name in available_admin:
                        summary.discovered[table.name] = len(
                            _read_rows(admin_source, table.name)
                        )
            return summary

        target_names = set(inspect(target).get_table_names())
        required_names = {table.name for table in sorted_tables()}
        missing = sorted(required_names - target_names)
        if missing:
            raise RuntimeError(
                "PostgreSQL schema is not at the migration head; missing tables: "
                + ", ".join(missing)
            )

        with target.begin() as connection:
            _copy_matching_tables(source, connection, server_tables, summary)
            _copy_client_events(source, connection, normalized_client_id, summary)
            if admin_source:
                _copy_matching_tables(
                    admin_source,
                    connection,
                    admin_metadata.sorted_tables,
                    summary,
                )
            _reset_integer_sequences(connection, sorted_tables())
        return summary
    finally:
        source.dispose()
        if admin_source:
            admin_source.dispose()


def _print_summary(summary: ImportSummary, dry_run: bool) -> None:
    label = "would import" if dry_run else "inserted"
    for table_name in sorted(summary.discovered):
        discovered = summary.discovered[table_name]
        inserted = summary.inserted.get(table_name, 0)
        count = discovered if dry_run else inserted
        print(f"{table_name}: {count} row(s) {label}")
    if not dry_run:
        print(f"total inserted: {summary.total_inserted}")


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Import legacy DevTrack SQLite history into PostgreSQL"
    )
    parser.add_argument("--source", type=Path, required=True, help="legacy devtrack.db")
    parser.add_argument("--client-id", required=True, help="owner of Go activity rows")
    parser.add_argument("--admin-source", type=Path, help="legacy separate admin.db")
    parser.add_argument("--dry-run", action="store_true", help="inspect without writing")
    args = parser.parse_args(argv)
    summary = import_sqlite(
        args.source,
        client_id=args.client_id,
        admin_source_path=args.admin_source,
        dry_run=args.dry_run,
    )
    _print_summary(summary, args.dry_run)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
