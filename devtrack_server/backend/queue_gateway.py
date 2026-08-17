"""
queue_gateway.py — Pending-actions staging layer for DevTrack Phase 1.

This is the ONLY place in the Python server that writes rows to the
``pending_actions`` table.  All PM-posting code must call
:meth:`QueueGateway.stage` instead of hitting a PM API directly.
The actual PM post is deferred until the Go daemon's queue executor
calls ``POST /queue/execute`` after the confidence timeout expires
(or the developer approves manually via TUI / CLI / Telegram).

Confidence-to-timeout rules (mirror of Go's ConfidenceTimeout in
devtrack_client/internal/db/pending_actions.go):

    is_new_action_type=True   → 30 minutes  (needs developer review)
    confidence > 0.90         → 2  minutes  (high confidence, short window)
    confidence >= 0.70        → 5  minutes  (moderate confidence)
    confidence < 0.70         → 15 minutes  (low confidence, longer review window)

The queue is a server-side persistence surface. TASK-114 registers its schema
on the shared SQLAlchemy metadata, so the same gateway works against mandatory
PostgreSQL and the isolated SQLite test backend. Client-originated queue rows
remain local-first and are copied separately through the opt-in client-event
outbox.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime, timedelta, timezone
from typing import Optional

from sqlalchemy import Column, Float, Integer, Table, Text, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata

logger = logging.getLogger(__name__)


class QueueGatewayUnavailableError(RuntimeError):
    """Deprecated compatibility error retained for callers importing it."""


pending_actions_table = Table(
    "pending_actions",
    metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("action_type", Text, nullable=False),
    Column("target", Text, nullable=False),
    Column("platform", Text, nullable=False),
    Column("workspace", Text, nullable=False),
    Column("payload", Text, nullable=False),
    Column("confidence", Float, nullable=False),
    Column("status", Text, nullable=False),
    Column("expires_at", Text, nullable=False),
    Column("created_at", Text, nullable=False),
    Column("acted_at", Text),
    Column("acted_by", Text),
    Column("error", Text),
)

def _init(engine: Optional[Engine] = None) -> Engine:
    eng = engine or get_engine()
    metadata.create_all(eng, tables=[pending_actions_table])
    return eng


def _now_str() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S")


def _confidence_timeout(confidence: float, is_new_action_type: bool) -> timedelta:
    """Return the review window for an action given its confidence score.

    Mirrors ``ConfidenceTimeout`` in
    ``devtrack_client/internal/db/pending_actions.go``.

    Rules (evaluated top-to-bottom — first match wins):
    - is_new_action_type=True  → 30 minutes
    - confidence > 0.90        → 2  minutes
    - confidence >= 0.70       → 5  minutes
    - confidence < 0.70        → 15 minutes
    """
    if is_new_action_type:
        return timedelta(minutes=30)
    if confidence > 0.90:
        return timedelta(minutes=2)
    if confidence >= 0.70:
        return timedelta(minutes=5)
    return timedelta(minutes=15)


class QueueGateway:
    """Writes rows to ``pending_actions`` via the shared SQLAlchemy engine.

    No connection is held across calls — each method opens
    ``backend.db.engine.get_engine().connect()`` for the duration of that
    call only. The schema and operations are dialect-neutral.

    Usage::

        from backend.queue_gateway import QueueGateway
        gw = QueueGateway()
        action_id = gw.stage(
            action_type="post_comment",
            target="PROJ-123",
            platform="github",
            workspace="my-workspace",
            payload={"comment": "Fixed null check in auth flow."},
            confidence=0.75,
        )
    """

    def __init__(self) -> None:
        """No-op — kept so existing call sites that construct with no args work
        unchanged.  The DB connection is opened lazily per call via
        ``backend.db.engine.get_engine()``; this class holds no state.
        """

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def stage(
        self,
        action_type: str,
        target: str,
        platform: str,
        workspace: str,
        payload: dict,
        confidence: float,
        is_new_action_type: bool = False,
    ) -> int:
        """Insert a new *pending* action row into ``pending_actions``.

        :param action_type: Verb describing the action, e.g. ``"post_comment"``,
            ``"state_transition"``, ``"eod_report"``.
        :param target: The PM object being acted upon, e.g. ``"PROJ-123"``,
            ``"PR #456"``, ``"ADO-789"``.
        :param platform: Destination platform: ``"github"``, ``"azure"``,
            ``"gitlab"``, or ``"jira"``.
        :param workspace: Workspace name from ``workspaces.yaml``.
        :param payload: Arbitrary dict that will be JSON-serialised and stored
            in the ``payload`` TEXT column.  The executor reads this when
            calling ``_execute_pm_action``.
        :param confidence: Float 0.0–1.0.  Drives the ``expires_at`` timeout
            via :func:`_confidence_timeout`.
        :param is_new_action_type: When ``True``, force the 30-minute review
            window regardless of ``confidence``.
        :returns: The ``id`` of the newly inserted row.
        """
        timeout = _confidence_timeout(confidence, is_new_action_type)
        expires_at = (datetime.now(timezone.utc) + timeout).strftime("%Y-%m-%dT%H:%M:%S")
        payload_json = json.dumps(payload)
        created_at = _now_str()
        with _init().begin() as conn:
            result = conn.execute(pending_actions_table.insert().values(
                action_type=action_type,
                target=target,
                platform=platform,
                workspace=workspace,
                payload=payload_json,
                confidence=confidence,
                status="pending",
                expires_at=expires_at,
                created_at=created_at,
            ))
            return int(result.inserted_primary_key[0])

    def mark_posted(self, action_id: int) -> None:
        """Mark *action_id* as successfully posted.

        Sets ``status = 'posted'``, ``acted_at = NOW()``, ``acted_by = 'auto'``.
        Called by the queue executor after a successful PM API call.

        """
        acted_at = _now_str()
        with _init().begin() as conn:
            conn.execute(
                pending_actions_table.update()
                .where(pending_actions_table.c.id == action_id)
                .values(status="posted", acted_at=acted_at, acted_by="auto")
            )

    def mark_failed(self, action_id: int, error: str) -> None:
        """Mark *action_id* as failed with an error message.

        Sets ``status = 'failed'``, ``error = error``, ``acted_at = NOW()``.
        Called by the queue executor when the PM API call raises an exception.

        """
        acted_at = _now_str()
        with _init().begin() as conn:
            conn.execute(
                pending_actions_table.update()
                .where(pending_actions_table.c.id == action_id)
                .values(status="failed", error=error, acted_at=acted_at)
            )

    # ------------------------------------------------------------------
    # Helpers used by /queue/pending endpoint
    # ------------------------------------------------------------------

    def list_pending(self) -> list[dict]:
        """Return all rows with ``status = 'pending'``, ordered by ``expires_at ASC``.

        Each row is returned as a plain ``dict`` (safe to serialise to JSON).

        """
        stmt = (
            select(pending_actions_table)
            .where(pending_actions_table.c.status == "pending")
            .order_by(pending_actions_table.c.expires_at.asc())
        )
        with _init().connect() as conn:
            rows = conn.execute(stmt).mappings().all()
        return [dict(row) for row in rows]

    def get_action(self, action_id: int) -> Optional[dict]:
        """Fetch a single row by *action_id*.

        Returns ``None`` when the row does not exist.

        """
        stmt = select(pending_actions_table).where(pending_actions_table.c.id == action_id)
        with _init().connect() as conn:
            row = conn.execute(stmt).mappings().fetchone()
        return dict(row) if row else None

    def close(self) -> None:
        """No-op — kept for backward compatibility with callers that call
        ``gw.close()`` (e.g. ``webhook_server.py``'s ``/queue/pending``
        endpoint).  There is no persistent connection to close: every method
        opens and releases its own connection via ``get_engine()``.
        """
