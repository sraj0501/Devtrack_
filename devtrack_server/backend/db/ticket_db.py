"""
TicketDB — SQLite helper for ticket_cache and pm_update_queue tables.

All callers obtain a TicketDB instance by calling TicketDB.from_config(), which
reads DATABASE_PATH from backend.config.  Never pass a hardcoded path.
"""

import sqlite3
from datetime import datetime
from typing import List, Optional

import backend.config as config


class TicketDB:
    """Thin SQLite wrapper scoped to the ticket_cache / pm_update_queue tables."""

    def __init__(self, db_path: str) -> None:
        self._path = db_path
        self._conn: Optional[sqlite3.Connection] = None

    # ------------------------------------------------------------------
    # Construction helpers
    # ------------------------------------------------------------------

    @classmethod
    def from_config(cls) -> "TicketDB":
        """Create a TicketDB using DATABASE_PATH from backend.config."""
        db_path = config.get("DATABASE_PATH")
        if not db_path:
            # Fall back to the composite path helper used by the rest of the backend.
            db_path = str(config.database_path())
        return cls(db_path)

    # ------------------------------------------------------------------
    # Connection management
    # ------------------------------------------------------------------

    def _get_conn(self) -> sqlite3.Connection:
        if self._conn is None:
            self._conn = sqlite3.connect(self._path)
            self._conn.row_factory = sqlite3.Row
        return self._conn

    def close(self) -> None:
        if self._conn is not None:
            self._conn.close()
            self._conn = None

    def __enter__(self) -> "TicketDB":
        return self

    def __exit__(self, *_) -> None:
        self.close()

    # ------------------------------------------------------------------
    # ticket_cache
    # ------------------------------------------------------------------

    def upsert_ticket(self, record: dict) -> None:
        """Insert or replace a ticket_cache row.

        Required keys: id, source, external_id, title, synced_at.
        Optional keys: repo, description, status, assignee, labels, url.
        """
        conn = self._get_conn()
        synced_at = record.get("synced_at")
        if isinstance(synced_at, datetime):
            synced_at = synced_at.strftime("%Y-%m-%d %H:%M:%S")

        conn.execute(
            """
            INSERT OR REPLACE INTO ticket_cache
                (id, source, external_id, repo, title, description,
                 status, assignee, labels, url, synced_at,
                 created_at)
            VALUES
                (:id, :source, :external_id, :repo, :title, :description,
                 :status, :assignee, :labels, :url, :synced_at,
                 COALESCE(
                     (SELECT created_at FROM ticket_cache WHERE id = :id),
                     CURRENT_TIMESTAMP
                 ))
            """,
            {
                "id": record["id"],
                "source": record["source"],
                "external_id": str(record["external_id"]),
                "repo": record.get("repo") or "",
                "title": record["title"],
                "description": record.get("description") or "",
                "status": record.get("status") or "",
                "assignee": record.get("assignee") or "",
                "labels": record.get("labels") or "[]",
                "url": record.get("url") or "",
                "synced_at": synced_at or datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S"),
            },
        )
        conn.commit()

    def get_tickets_by_assignee(self, assignee: str) -> List[dict]:
        """Return all cached tickets for *assignee*, most-recently synced first."""
        conn = self._get_conn()
        rows = conn.execute(
            """
            SELECT id, source, external_id, repo, title, description,
                   status, assignee, labels, url, synced_at, created_at
            FROM ticket_cache
            WHERE assignee = ?
            ORDER BY synced_at DESC
            """,
            (assignee,),
        ).fetchall()
        return [dict(r) for r in rows]

    def get_ticket_by_id(self, ticket_id: str) -> Optional[dict]:
        """Return a single cached ticket dict or None."""
        conn = self._get_conn()
        row = conn.execute(
            "SELECT * FROM ticket_cache WHERE id = ?", (ticket_id,)
        ).fetchone()
        return dict(row) if row else None

    def get_all_tickets(self) -> List[dict]:
        """Return all rows from ticket_cache."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM ticket_cache ORDER BY synced_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]
