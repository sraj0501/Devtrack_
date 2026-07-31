"""
dialectic_status.py — Read-only summary helpers for Phase 6 dialectic data.

Queries the shared SQLite DB (devtrack.db) for inferences, skills, and
confidence-threshold data so the /voice/status endpoint can surface them.

All three public methods return empty/zero values on any DB error — they
never raise (graceful degradation — Non-Negotiable #8).

Config: db_path resolved via backend.config.database_path() —
no os.getenv calls anywhere in this module.

Boundary rule
-------------
``inferences``, ``corrections``, ``skills``, and ``confidence_thresholds``
are Go-owned tables (created and written by the Go daemon — see
``devtrack_client/internal/db/migrations.go`` and ``database.go``).  This
module never defines a SQLAlchemy ``Table`` for any of them and never runs
DDL against them — Python must not create or own their schema (see the
PostgreSQL Backend epic, ``Data/agent_logs/project_board.md``).

  SQLite mode     (POSTGRES_URL unset) — all four tables live in the same
    devtrack.db file the dual-dialect engine already points at, so reads go
    through ``backend.db.engine.get_engine()`` via SQLAlchemy ``text()``
    instead of a bespoke ``sqlite3.connect()``.
  PostgreSQL mode (POSTGRES_URL set)   — Go never speaks Postgres (decided
    2026-07-13), so none of these four tables exist there and there is no Go
    internal-HTTP endpoint exposing them yet. Every method fails closed
    immediately in this mode — safe default, never raise, never queried.
"""

from __future__ import annotations

import logging

from sqlalchemy import text

from backend.config import database_path
from backend.db.engine import get_engine, is_postgres

logger = logging.getLogger("devtrack.dialectic_status")


class DialecticStatus:
    """Read-only summaries of dialectic Phase 6 data from the shared SQLite DB.

    Usage::

        ds = DialecticStatus()
        inferences = ds.get_inference_summary()   # {total, top_by_confidence, correction_count}
        skills      = ds.get_skill_summary()       # {total, names}
        thresholds  = ds.get_threshold_summary()   # {action_type: {threshold, approvals, rejections}}

    The DB path is resolved lazily on each method call so that DB errors are
    contained within each method's own try/except and do not propagate to callers.
    """

    def __init__(self) -> None:
        pass  # DB path resolved lazily per method call

    def _resolve_db_path(self):
        """Resolve and return the SQLite DB path.  Raises on config error."""
        return database_path()

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def get_inference_summary(self) -> dict:
        """Return a summary of dialectic inferences.

        Returns::

            {
              "total": 42,
              "top_by_confidence": [
                {"id": 7, "subject": "commit tone", "inference": "...",
                 "confidence": 0.91, "context_type": "commit"},
                ...   (at most 5 entries, ordered by confidence DESC)
              ],
              "correction_count": 2
            }

        Returns ``{"total": 0, "top_by_confidence": [], "correction_count": 0}``
        on any DB error, or immediately in PostgreSQL mode (see the
        boundary-rule note at the top of this module).
        """
        _safe = {"total": 0, "top_by_confidence": [], "correction_count": 0}
        if is_postgres():
            logger.debug(
                "dialectic_status: get_inference_summary is a no-op in PostgreSQL "
                "mode — inferences/corrections are Go-owned SQLite-only tables "
                "with no Postgres equivalent yet"
            )
            return _safe
        try:
            db_path = self._resolve_db_path()
            if not db_path.exists():
                return _safe
            with get_engine().connect() as conn:
                # Total inferences.
                row = conn.execute(text("SELECT COUNT(*) FROM inferences")).fetchone()
                total = int(row[0]) if row else 0

                # Top 5 by confidence DESC.
                rows = conn.execute(
                    text(
                        """
                        SELECT id, subject, inference, confidence, context_type
                          FROM inferences
                         ORDER BY confidence DESC
                         LIMIT 5
                        """
                    )
                ).mappings().all()
                top = [
                    {
                        "id": int(r["id"]),
                        "subject": r["subject"],
                        "inference": r["inference"],
                        "confidence": float(r["confidence"]),
                        "context_type": r["context_type"],
                    }
                    for r in rows
                ]

                # Count developer corrections.
                row = conn.execute(text("SELECT COUNT(*) FROM corrections")).fetchone()
                correction_count = int(row[0]) if row else 0

            return {
                "total": total,
                "top_by_confidence": top,
                "correction_count": correction_count,
            }

        except Exception as exc:
            logger.debug("dialectic_status: get_inference_summary failed (non-fatal): %s", exc)
            return _safe

    def get_skill_summary(self) -> dict:
        """Return a summary of autonomously promoted skills.

        Returns::

            {"total": 3, "names": ["imperative_commit_tone", "bracket_ticket_prefix", ...]}

        Returns ``{"total": 0, "names": []}`` on any DB error, if the skills
        table does not yet exist (TASK-089 may not be merged yet), or
        immediately in PostgreSQL mode (see the boundary-rule note at the
        top of this module).
        """
        _safe: dict = {"total": 0, "names": []}
        if is_postgres():
            logger.debug(
                "dialectic_status: get_skill_summary is a no-op in PostgreSQL "
                "mode — skills is a Go-owned SQLite-only table with no "
                "Postgres equivalent yet"
            )
            return _safe
        try:
            db_path = self._resolve_db_path()
            if not db_path.exists():
                return _safe
            with get_engine().connect() as conn:
                # Check whether the table exists first — TASK-089 may not be merged yet.
                table_exists = conn.execute(
                    text(
                        "SELECT COUNT(*) FROM sqlite_master "
                        "WHERE type='table' AND name='skills'"
                    )
                ).fetchone()[0]
                if not table_exists:
                    return _safe

                rows = conn.execute(
                    text("SELECT name FROM skills ORDER BY promoted_at ASC")
                ).fetchall()
                names = [r[0] for r in rows]
            return {"total": len(names), "names": names}

        except Exception as exc:
            logger.debug("dialectic_status: get_skill_summary failed (non-fatal): %s", exc)
            return _safe

    def get_threshold_summary(self) -> dict:
        """Return per-action-type confidence thresholds.

        Returns::

            {
              "post_comment":     {"threshold": 0.86, "approvals": 43, "rejections": 4},
              "state_transition": {"threshold": 0.82, "approvals": 21, "rejections": 7},
              ...
            }

        Returns ``{}`` on any DB error, if the table does not yet exist, or
        immediately in PostgreSQL mode (see the boundary-rule note at the
        top of this module).
        """
        _safe: dict = {}
        if is_postgres():
            logger.debug(
                "dialectic_status: get_threshold_summary is a no-op in "
                "PostgreSQL mode — confidence_thresholds is a Go-owned "
                "SQLite-only table with no Postgres equivalent yet"
            )
            return _safe
        try:
            db_path = self._resolve_db_path()
            if not db_path.exists():
                return _safe
            with get_engine().connect() as conn:
                # Check table exists.
                table_exists = conn.execute(
                    text(
                        "SELECT COUNT(*) FROM sqlite_master "
                        "WHERE type='table' AND name='confidence_thresholds'"
                    )
                ).fetchone()[0]
                if not table_exists:
                    return _safe

                rows = conn.execute(
                    text(
                        """
                        SELECT action_type, threshold, approvals, rejections
                          FROM confidence_thresholds
                         ORDER BY action_type ASC
                        """
                    )
                ).mappings().all()

            result: dict = {}
            for r in rows:
                result[r["action_type"]] = {
                    "threshold": float(r["threshold"]),
                    "approvals": int(r["approvals"]),
                    "rejections": int(r["rejections"]),
                }
            return result

        except Exception as exc:
            logger.debug("dialectic_status: get_threshold_summary failed (non-fatal): %s", exc)
            return _safe
