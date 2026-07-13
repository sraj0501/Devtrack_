"""
queue_gateway.py — Pending-actions staging layer for DevTrack Phase 1.

This is the ONLY place in the Python server that writes rows to the
``pending_actions`` SQLite table.  All PM-posting code must call
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

Config: db_path is resolved via ``backend.config.database_path()`` — never
via ``os.getenv`` directly.
"""

from __future__ import annotations

import json
import sqlite3
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional


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
    """Writes rows to ``pending_actions`` in the shared DevTrack SQLite DB.

    The connection is opened once at construction time and reused across
    calls.  Thread-safety relies on SQLite's default serialised mode; if
    concurrent writes are needed, instantiate one gateway per thread or
    switch to ``check_same_thread=False`` with an external lock.

    Usage::

        from backend.config import database_path
        gw = QueueGateway(str(database_path()))
        action_id = gw.stage(
            action_type="post_comment",
            target="PROJ-123",
            platform="github",
            workspace="my-workspace",
            payload={"comment": "Fixed null check in auth flow."},
            confidence=0.75,
        )
    """

    def __init__(self, db_path: str) -> None:
        """Open a SQLite connection to *db_path*.

        The file (and any parent directories) must already exist; this
        class does NOT run migrations.  The Go daemon creates and migrates
        the DB on startup, so by the time the Python server stages its
        first action the table is guaranteed to be present.

        :param db_path: Absolute path to ``devtrack.db``.  Resolve via
            ``backend.config.database_path()``.
        """
        self._db_path = db_path
        # isolation_level=None → autocommit mode so each write is durable
        # immediately; the Go executor reads across process boundaries.
        self._conn = sqlite3.connect(db_path, check_same_thread=False)
        self._conn.row_factory = sqlite3.Row

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
        :returns: The ``id`` (ROWID) of the newly inserted row.
        """
        timeout = _confidence_timeout(confidence, is_new_action_type)
        expires_at = (datetime.utcnow() + timeout).strftime("%Y-%m-%dT%H:%M:%S")
        payload_json = json.dumps(payload)

        cur = self._conn.execute(
            """
            INSERT INTO pending_actions
                (action_type, target, platform, workspace, payload,
                 confidence, status, expires_at)
            VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)
            """,
            (action_type, target, platform, workspace, payload_json,
             confidence, expires_at),
        )
        self._conn.commit()
        return cur.lastrowid  # type: ignore[return-value]

    def mark_posted(self, action_id: int) -> None:
        """Mark *action_id* as successfully posted.

        Sets ``status = 'posted'``, ``acted_at = NOW()``, ``acted_by = 'auto'``.
        Called by the queue executor after a successful PM API call.
        """
        acted_at = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S")
        self._conn.execute(
            """
            UPDATE pending_actions
               SET status    = 'posted',
                   acted_at  = ?,
                   acted_by  = 'auto'
             WHERE id = ?
            """,
            (acted_at, action_id),
        )
        self._conn.commit()

    def mark_failed(self, action_id: int, error: str) -> None:
        """Mark *action_id* as failed with an error message.

        Sets ``status = 'failed'``, ``error = error``, ``acted_at = NOW()``.
        Called by the queue executor when the PM API call raises an exception.
        """
        acted_at = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S")
        self._conn.execute(
            """
            UPDATE pending_actions
               SET status   = 'failed',
                   error    = ?,
                   acted_at = ?
             WHERE id = ?
            """,
            (error, acted_at, action_id),
        )
        self._conn.commit()

    # ------------------------------------------------------------------
    # Helpers used by /queue/pending endpoint
    # ------------------------------------------------------------------

    def list_pending(self) -> list[dict]:
        """Return all rows with ``status = 'pending'``, ordered by ``expires_at ASC``.

        Each row is returned as a plain ``dict`` (safe to serialise to JSON).
        """
        cur = self._conn.execute(
            """
            SELECT id, action_type, target, platform, workspace, payload,
                   confidence, status, expires_at, created_at,
                   acted_at, acted_by, error
              FROM pending_actions
             WHERE status = 'pending'
             ORDER BY expires_at ASC
            """
        )
        rows = cur.fetchall()
        return [dict(row) for row in rows]

    def get_action(self, action_id: int) -> Optional[dict]:
        """Fetch a single row by *action_id*.

        Returns ``None`` when the row does not exist.
        """
        cur = self._conn.execute(
            """
            SELECT id, action_type, target, platform, workspace, payload,
                   confidence, status, expires_at, created_at,
                   acted_at, acted_by, error
              FROM pending_actions
             WHERE id = ?
            """,
            (action_id,),
        )
        row = cur.fetchone()
        return dict(row) if row else None

    def close(self) -> None:
        """Close the underlying SQLite connection."""
        self._conn.close()
