"""
report_store.py — SQLAlchemy persistence for the daily/weekly ``reports`` table.

Runs against SQLite (local) or PostgreSQL (multi-user), selected via
POSTGRES_URL — same dual-dialect engine every other Python-owned store
module uses (see ``backend.db.engine``).

Python fully owns this table's schema: ``backend.daily_report_generator.
DailyReportGenerator`` is the sole reader/writer (confirmed via a repo-wide
grep for ``FROM reports`` / ``INTO reports`` — the only other files
mentioning the word "reports" are ``reports_dir()``-style filesystem paths
in ``email_reporter.py``/``config.py``, an unrelated concept). Because
Python owns this schema outright, unlike the Go-owned tables ported
elsewhere in the Postgres backend epic (TASK-112), this table gets a real
``Table`` registered on the shared ``metadata`` and works identically in
both SQLite and PostgreSQL mode — no boundary-rule fail-closed guard is
needed here.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from sqlalchemy import Column, Float, Integer, Table, Text, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Table definition
# ---------------------------------------------------------------------------
# Columns/constraints match the original
# ``CREATE TABLE IF NOT EXISTS reports (...)`` DDL from
# daily_report_generator.py verbatim, just expressed as SQLAlchemy Core.

reports_table = Table(
    "reports", metadata,
    Column("id",              Integer, primary_key=True, autoincrement=True),
    Column("report_date",     Text, nullable=False),
    Column("report_type",     Text, nullable=False),
    Column("format",          Text, nullable=False),
    Column("content",         Text, nullable=False),
    Column("summary",         Text),
    Column("total_hours",     Float),
    Column("task_count",      Integer),
    Column("completed_count", Integer),
    Column("projects_count",  Integer),
    Column("ai_enhanced",     Integer),
    Column("email_sent",      Integer),
    Column("email_sent_at",   Text),
    Column("created_at",      Text, nullable=False),
)

_schema_done: bool = False


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if engine is None and _schema_done:
        return eng
    metadata.create_all(eng, tables=[reports_table])
    if engine is None:
        _schema_done = True
    return eng


def _now_str() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def insert_report(row: Dict[str, Any], engine: Optional[Engine] = None) -> Optional[int]:
    """Insert a new row into ``reports``. Plain insert — no upsert semantics,
    matching the original raw-SQL ``INSERT`` (there was never an
    update-existing-row path here; only ``mark_email_sent`` updates a row
    after the fact).

    ``row`` must contain: report_date, report_type, format, content, summary,
    total_hours, task_count, completed_count, projects_count, ai_enhanced.
    ``email_sent`` defaults to False and ``created_at`` is stamped here if
    not supplied.

    Returns the new row's id, or None on any DB error (never raises —
    matches the original's ``except Exception: return None``).
    """
    eng = _init(engine)
    values = dict(row)
    values.setdefault("email_sent", 0)
    values.setdefault("email_sent_at", None)
    values.setdefault("created_at", _now_str())
    if "ai_enhanced" in values:
        values["ai_enhanced"] = int(bool(values["ai_enhanced"]))
    try:
        with eng.begin() as conn:
            result = conn.execute(reports_table.insert().values(**values))
            pk = result.inserted_primary_key
            return int(pk[0]) if pk else None
    except Exception as exc:
        logger.warning("report_store.insert_report failed: %s", exc)
        return None


def mark_email_sent(
    report_id: int, sent: bool = True, engine: Optional[Engine] = None
) -> bool:
    """Update ``reports.email_sent``/``email_sent_at`` for *report_id*.

    Returns True on success, False on any DB error — matches the original's
    behaviour of never checking whether the WHERE clause actually matched a
    row (a not-found id is not treated as a failure, same as before).
    """
    eng = _init(engine)
    try:
        values = (
            {"email_sent": 1, "email_sent_at": _now_str()}
            if sent
            else {"email_sent": 0, "email_sent_at": None}
        )
        with eng.begin() as conn:
            conn.execute(
                reports_table.update()
                .where(reports_table.c.id == report_id)
                .values(**values)
            )
        return True
    except Exception as exc:
        logger.warning("report_store.mark_email_sent failed: %s", exc)
        return False


def get_reports(
    report_type: Optional[str] = None,
    cutoff_date: str = "",
    limit: int = 100,
    engine: Optional[Engine] = None,
) -> List[Dict[str, Any]]:
    """Return report rows with ``report_date >= cutoff_date`` (and matching
    ``report_type`` if given), most recent first. Empty list on any DB error.
    """
    eng = _init(engine)
    try:
        stmt = select(reports_table)
        if cutoff_date:
            stmt = stmt.where(reports_table.c.report_date >= cutoff_date)
        if report_type:
            stmt = stmt.where(reports_table.c.report_type == report_type)
        stmt = stmt.order_by(
            reports_table.c.report_date.desc(), reports_table.c.created_at.desc()
        ).limit(limit)
        with eng.connect() as conn:
            rows = conn.execute(stmt).mappings().all()
        return [dict(r) for r in rows]
    except Exception as exc:
        logger.warning("report_store.get_reports failed: %s", exc)
        return []


def get_content(report_id: int, engine: Optional[Engine] = None) -> Optional[str]:
    """Return the ``content`` column for *report_id*, or None (missing row or
    any DB error)."""
    eng = _init(engine)
    try:
        with eng.connect() as conn:
            row = conn.execute(
                select(reports_table.c.content).where(reports_table.c.id == report_id)
            ).first()
        return row[0] if row else None
    except Exception as exc:
        logger.warning("report_store.get_content failed: %s", exc)
        return None
