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

Boundary rule
-------------
``pending_actions`` is a Go-owned table (created and migrated by the Go
daemon — see ``devtrack_client/internal/db/pending_actions.go`` and
``migrations.go``).  Unlike every other Go-owned table ported so far, this
one is *written* by Python (``stage()`` is the only Python-side write path
into the queue), not just read.

  SQLite mode     (POSTGRES_URL unset) — all methods use the shared
    SQLAlchemy engine (``backend.db.engine.get_engine()``) instead of a
    bespoke ``sqlite3.connect()``.  The engine is a lazily-built,
    process-wide singleton with its own pooling, so this class holds no
    connection of its own — every method opens (and releases) a connection
    per call.
  PostgreSQL mode (POSTGRES_URL set)   — there is no Go internal-HTTP
    endpoint for staging or reading the queue today (confirmed against
    ``devtrack_client/internal/daemon/http_api.go`` — it exposes
    ``/internal/force-trigger``, ``/internal/reload-config``,
    ``/internal/stats``, and ``/internal/sessions/active``, nothing
    queue-related), and building one is real, unstarted design work
    (tracked separately as TASK-114, client→server sync path). Every public
    method below raises :class:`QueueGatewayUnavailableError` immediately in
    this mode — loudly, not a silent empty/no-op default — because a
    swallowed failure here used to trigger an unreviewed direct PM post
    fallback in ``webhook_server.py`` (removed as part of this same change;
    see ``TriggerProcessor.process_commit``). Callers must catch this
    themselves if they want graceful degradation.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime, timedelta
from typing import Optional

from sqlalchemy import text

from backend.db.engine import get_engine, is_postgres

logger = logging.getLogger(__name__)


class QueueGatewayUnavailableError(RuntimeError):
    """pending_actions queue staging has no PostgreSQL-mode implementation yet (TASK-114)."""


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
    call only.  In PostgreSQL mode every method raises
    :class:`QueueGatewayUnavailableError` before touching the engine at all
    (see the boundary-rule note at the top of this module).

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
        :raises QueueGatewayUnavailableError: In PostgreSQL mode.
        """
        if is_postgres():
            raise QueueGatewayUnavailableError(
                "QueueGateway.stage: pending_actions has no PostgreSQL-mode "
                "implementation yet (TASK-114) — the queue cannot be staged to."
            )

        timeout = _confidence_timeout(confidence, is_new_action_type)
        expires_at = (datetime.utcnow() + timeout).strftime("%Y-%m-%dT%H:%M:%S")
        payload_json = json.dumps(payload)

        stmt = text(
            """
            INSERT INTO pending_actions
                (action_type, target, platform, workspace, payload,
                 confidence, status, expires_at)
            VALUES (:action_type, :target, :platform, :workspace, :payload,
                    :confidence, 'pending', :expires_at)
            """
        )
        with get_engine().connect() as conn:
            result = conn.execute(
                stmt,
                {
                    "action_type": action_type,
                    "target": target,
                    "platform": platform,
                    "workspace": workspace,
                    "payload": payload_json,
                    "confidence": confidence,
                    "expires_at": expires_at,
                },
            )
            conn.commit()
            return result.lastrowid  # type: ignore[return-value]

    def mark_posted(self, action_id: int) -> None:
        """Mark *action_id* as successfully posted.

        Sets ``status = 'posted'``, ``acted_at = NOW()``, ``acted_by = 'auto'``.
        Called by the queue executor after a successful PM API call.

        :raises QueueGatewayUnavailableError: In PostgreSQL mode.
        """
        if is_postgres():
            raise QueueGatewayUnavailableError(
                "QueueGateway.mark_posted: pending_actions has no PostgreSQL-mode "
                "implementation yet (TASK-114)."
            )

        acted_at = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S")
        stmt = text(
            """
            UPDATE pending_actions
               SET status    = 'posted',
                   acted_at  = :acted_at,
                   acted_by  = 'auto'
             WHERE id = :id
            """
        )
        with get_engine().connect() as conn:
            conn.execute(stmt, {"acted_at": acted_at, "id": action_id})
            conn.commit()

    def mark_failed(self, action_id: int, error: str) -> None:
        """Mark *action_id* as failed with an error message.

        Sets ``status = 'failed'``, ``error = error``, ``acted_at = NOW()``.
        Called by the queue executor when the PM API call raises an exception.

        :raises QueueGatewayUnavailableError: In PostgreSQL mode.
        """
        if is_postgres():
            raise QueueGatewayUnavailableError(
                "QueueGateway.mark_failed: pending_actions has no PostgreSQL-mode "
                "implementation yet (TASK-114)."
            )

        acted_at = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S")
        stmt = text(
            """
            UPDATE pending_actions
               SET status   = 'failed',
                   error    = :error,
                   acted_at = :acted_at
             WHERE id = :id
            """
        )
        with get_engine().connect() as conn:
            conn.execute(stmt, {"error": error, "acted_at": acted_at, "id": action_id})
            conn.commit()

    # ------------------------------------------------------------------
    # Helpers used by /queue/pending endpoint
    # ------------------------------------------------------------------

    def list_pending(self) -> list[dict]:
        """Return all rows with ``status = 'pending'``, ordered by ``expires_at ASC``.

        Each row is returned as a plain ``dict`` (safe to serialise to JSON).

        :raises QueueGatewayUnavailableError: In PostgreSQL mode.
        """
        if is_postgres():
            raise QueueGatewayUnavailableError(
                "QueueGateway.list_pending: pending_actions has no PostgreSQL-mode "
                "implementation yet (TASK-114)."
            )

        stmt = text(
            """
            SELECT id, action_type, target, platform, workspace, payload,
                   confidence, status, expires_at, created_at,
                   acted_at, acted_by, error
              FROM pending_actions
             WHERE status = 'pending'
             ORDER BY expires_at ASC
            """
        )
        with get_engine().connect() as conn:
            rows = conn.execute(stmt).mappings().all()
        return [dict(row) for row in rows]

    def get_action(self, action_id: int) -> Optional[dict]:
        """Fetch a single row by *action_id*.

        Returns ``None`` when the row does not exist.

        :raises QueueGatewayUnavailableError: In PostgreSQL mode.
        """
        if is_postgres():
            raise QueueGatewayUnavailableError(
                "QueueGateway.get_action: pending_actions has no PostgreSQL-mode "
                "implementation yet (TASK-114)."
            )

        stmt = text(
            """
            SELECT id, action_type, target, platform, workspace, payload,
                   confidence, status, expires_at, created_at,
                   acted_at, acted_by, error
              FROM pending_actions
             WHERE id = :id
            """
        )
        with get_engine().connect() as conn:
            row = conn.execute(stmt, {"id": action_id}).mappings().fetchone()
        return dict(row) if row else None

    def close(self) -> None:
        """No-op — kept for backward compatibility with callers that call
        ``gw.close()`` (e.g. ``webhook_server.py``'s ``/queue/pending``
        endpoint).  There is no persistent connection to close: every method
        opens and releases its own connection via ``get_engine()``.
        """
