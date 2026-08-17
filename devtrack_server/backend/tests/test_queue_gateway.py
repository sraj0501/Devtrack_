"""
Tests for QueueGateway (backend/queue_gateway.py) and the /queue/* endpoints.

queue_gateway.py no longer holds its own sqlite3 connection — every method
opens the shared SQLAlchemy engine (backend.db.engine.get_engine()) per
call, and the constructor takes no path argument. Tests therefore point
DATABASE_DIR at a fresh temp directory and reset the engine singleton per
test (same isolated-engine pattern as test_skill_detector.py /
test_server_tui.py / test_work_tracker.py), then create devtrack.db (the
default filename backend.config.database_path() resolves to) directly with
the pending_actions table DDL so the tests do not depend on the Go daemon
having run migrations first. The DDL is inlined here (same as Go migration
006).

Raw sqlite3 connections are still used within individual tests to assert on
row contents — that's fine, they just point at the same devtrack.db file the
engine-backed QueueGateway methods read/write.
"""

from __future__ import annotations

import json
import os
import sqlite3
import sys
from datetime import datetime, timedelta
from pathlib import Path
from unittest.mock import patch, MagicMock, AsyncMock

import pytest

# Ensure project root is on sys.path
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Table DDL (mirrors Go migration 006-create-pending-actions)
# ---------------------------------------------------------------------------

_CREATE_TABLE_SQL = """
CREATE TABLE IF NOT EXISTS pending_actions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    action_type TEXT    NOT NULL,
    target      TEXT    NOT NULL,
    platform    TEXT    NOT NULL,
    workspace   TEXT    NOT NULL,
    payload     TEXT    NOT NULL,
    confidence  REAL    NOT NULL,
    status      TEXT    NOT NULL DEFAULT 'pending',
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    acted_at    DATETIME,
    acted_by    TEXT,
    error       TEXT
);
CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at);
"""


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def isolated_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton so QueueGateway's engine-based reads/writes
    hit an isolated SQLite file instead of whatever engine a prior test
    built (same fixture shape as test_skill_detector.py's isolated_engine
    and test_work_tracker.py's TestWorkSessionStore.setup).
    """
    from backend.db.engine import reset_engine

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield
    reset_engine()


@pytest.fixture()
def db_path(tmp_path) -> str:
    """Return the path to a fresh SQLite DB with the pending_actions table.

    Filename must match backend.config.database_path()'s default
    ("devtrack.db") since QueueGateway no longer takes a path argument — it
    resolves its own path via backend.db.engine.get_engine(), same as
    production code.
    """
    path = str(tmp_path / "devtrack.db")
    conn = sqlite3.connect(path)
    conn.executescript(_CREATE_TABLE_SQL)
    conn.commit()
    conn.close()
    return path


@pytest.fixture()
def gateway(db_path):
    """Return a QueueGateway backed by the temp test DB (via the shared engine)."""
    from backend.queue_gateway import QueueGateway
    gw = QueueGateway()
    yield gw
    gw.close()


# ---------------------------------------------------------------------------
# Unit tests — QueueGateway.stage()
# ---------------------------------------------------------------------------

class TestStageInsertsPendingRow:
    def test_returns_integer_id(self, gateway):
        action_id = gateway.stage(
            action_type="post_comment",
            target="PROJ-123",
            platform="github",
            workspace="my-ws",
            payload={"comment": "hello"},
            confidence=0.75,
        )
        assert isinstance(action_id, int)
        assert action_id > 0

    def test_row_has_pending_status(self, gateway, db_path):
        action_id = gateway.stage(
            action_type="post_comment",
            target="PROJ-123",
            platform="github",
            workspace="my-ws",
            payload={"comment": "hello"},
            confidence=0.75,
        )
        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT status FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()
        assert row is not None
        assert row[0] == "pending"

    def test_payload_stored_as_json(self, gateway, db_path):
        payload = {"comment": "Fixed null check", "ticket_id": "GH-42"}
        action_id = gateway.stage(
            action_type="post_comment",
            target="GH-42",
            platform="github",
            workspace="ws",
            payload=payload,
            confidence=0.80,
        )
        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT payload FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()
        stored = json.loads(row[0])
        assert stored == payload

    def test_expires_at_high_confidence(self, gateway, db_path):
        """confidence > 0.90 → ~2-minute window."""
        before = datetime.utcnow()
        action_id = gateway.stage(
            action_type="post_comment",
            target="X",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.95,
        )
        after = datetime.utcnow()

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT expires_at FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        expires = datetime.fromisoformat(row[0])
        # Expect roughly now + 2 minutes (allow ±10s margin)
        expected_min = before + timedelta(minutes=2) - timedelta(seconds=10)
        expected_max = after + timedelta(minutes=2) + timedelta(seconds=10)
        assert expected_min <= expires <= expected_max

    def test_expires_at_moderate_confidence(self, gateway, db_path):
        """0.70 <= confidence <= 0.90 → ~5-minute window."""
        before = datetime.utcnow()
        action_id = gateway.stage(
            action_type="post_comment",
            target="X",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.75,
        )
        after = datetime.utcnow()

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT expires_at FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        expires = datetime.fromisoformat(row[0])
        expected_min = before + timedelta(minutes=5) - timedelta(seconds=10)
        expected_max = after + timedelta(minutes=5) + timedelta(seconds=10)
        assert expected_min <= expires <= expected_max

    def test_expires_at_low_confidence(self, gateway, db_path):
        """confidence < 0.70 → ~15-minute window."""
        before = datetime.utcnow()
        action_id = gateway.stage(
            action_type="post_comment",
            target="X",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.50,
        )
        after = datetime.utcnow()

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT expires_at FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        expires = datetime.fromisoformat(row[0])
        expected_min = before + timedelta(minutes=15) - timedelta(seconds=10)
        expected_max = after + timedelta(minutes=15) + timedelta(seconds=10)
        assert expected_min <= expires <= expected_max

    def test_expires_at_new_action_type(self, gateway, db_path):
        """is_new_action_type=True → ~30-minute window regardless of confidence."""
        before = datetime.utcnow()
        action_id = gateway.stage(
            action_type="brand_new_verb",
            target="X",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.99,
            is_new_action_type=True,
        )
        after = datetime.utcnow()

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT expires_at FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        expires = datetime.fromisoformat(row[0])
        expected_min = before + timedelta(minutes=30) - timedelta(seconds=10)
        expected_max = after + timedelta(minutes=30) + timedelta(seconds=10)
        assert expected_min <= expires <= expected_max


# ---------------------------------------------------------------------------
# Unit tests — QueueGateway.mark_posted()
# ---------------------------------------------------------------------------

class TestMarkPosted:
    def test_status_becomes_posted(self, gateway, db_path):
        action_id = gateway.stage(
            action_type="post_comment",
            target="T",
            platform="azure",
            workspace="ws",
            payload={},
            confidence=0.80,
        )
        gateway.mark_posted(action_id)

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT status, acted_by FROM pending_actions WHERE id = ?",
            (action_id,),
        )
        row = cur.fetchone()
        conn.close()

        assert row[0] == "posted"
        assert row[1] == "auto"

    def test_acted_at_is_set(self, gateway, db_path):
        action_id = gateway.stage(
            action_type="post_comment",
            target="T",
            platform="azure",
            workspace="ws",
            payload={},
            confidence=0.80,
        )
        before = datetime.utcnow()
        gateway.mark_posted(action_id)
        after = datetime.utcnow()

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT acted_at FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        acted_at = datetime.fromisoformat(row[0])
        assert before - timedelta(seconds=2) <= acted_at <= after + timedelta(seconds=2)


# ---------------------------------------------------------------------------
# Unit tests — QueueGateway.mark_failed()
# ---------------------------------------------------------------------------

class TestMarkFailed:
    def test_status_becomes_failed(self, gateway, db_path):
        action_id = gateway.stage(
            action_type="post_comment",
            target="T",
            platform="jira",
            workspace="ws",
            payload={},
            confidence=0.65,
        )
        gateway.mark_failed(action_id, "Connection refused")

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT status, error FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        assert row[0] == "failed"
        assert row[1] == "Connection refused"

    def test_error_field_populated(self, gateway, db_path):
        action_id = gateway.stage(
            action_type="state_transition",
            target="ADO-789",
            platform="azure",
            workspace="ws",
            payload={},
            confidence=0.55,
        )
        error_msg = "HTTP 429: rate limit exceeded"
        gateway.mark_failed(action_id, error_msg)

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT error FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()

        assert row[0] == error_msg


# ---------------------------------------------------------------------------
# Unit tests — _confidence_timeout helper
# ---------------------------------------------------------------------------

class TestConfidenceTimeout:
    def test_new_action_type_30_min(self):
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.99, is_new_action_type=True)
        assert dt == timedelta(minutes=30)

    def test_high_confidence_2_min(self):
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.91, is_new_action_type=False)
        assert dt == timedelta(minutes=2)

    def test_moderate_confidence_5_min(self):
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.70, is_new_action_type=False)
        assert dt == timedelta(minutes=5)

    def test_low_confidence_15_min(self):
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.69, is_new_action_type=False)
        assert dt == timedelta(minutes=15)

    def test_zero_confidence_15_min(self):
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.0, is_new_action_type=False)
        assert dt == timedelta(minutes=15)

    def test_exact_0_90_boundary(self):
        """confidence == 0.90 is NOT > 0.90 so it should fall into the 5-min bucket."""
        from backend.queue_gateway import _confidence_timeout
        dt = _confidence_timeout(0.90, is_new_action_type=False)
        assert dt == timedelta(minutes=5)


# ---------------------------------------------------------------------------
# Dialect-neutral schema behaviour
# ---------------------------------------------------------------------------

class TestQueueGatewayDialectNeutral:
    def test_no_postgres_fail_closed_guard_remains(self):
        import backend.queue_gateway as module

        assert not hasattr(module, "is_postgres")

    def test_close_is_safe(self):
        from backend.queue_gateway import QueueGateway

        QueueGateway().close()


# ---------------------------------------------------------------------------
# Integration smoke tests — /queue/pending endpoint via FastAPI TestClient
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup_queue_tests():
    """Stub blocking lifespan calls for the whole test module."""
    noop = AsyncMock(return_value=None)
    with (
        patch("backend.webhook_server.TriggerProcessor._init_components"),
        patch("backend.webhook_server._ensure_gitlab_webhooks", new=noop),
    ):
        yield


@pytest.fixture(autouse=True)
def _reset_trigger_processor():
    """Reset TriggerProcessor singleton between tests."""
    from backend.webhook_server import TriggerProcessor
    TriggerProcessor._instance = None
    yield
    TriggerProcessor._instance = None


@pytest.fixture()
def api_client(monkeypatch):
    """FastAPI TestClient with no auth required and TLS disabled."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


@pytest.fixture()
def api_client_with_queue(monkeypatch, db_path):
    """FastAPI TestClient with a real queue gateway pointing at the temp DB."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")

    # Patch _get_queue_gateway to return a real gateway backed by our temp DB
    from backend.queue_gateway import QueueGateway
    gw_instance = QueueGateway()

    with patch("backend.webhook_server._get_queue_gateway", return_value=gw_instance):
        from fastapi.testclient import TestClient
        from backend.webhook_server import app
        client = TestClient(app, raise_server_exceptions=True)
        yield client

    gw_instance.close()


class TestGetQueuePendingEndpoint:
    def test_returns_200(self, api_client):
        resp = api_client.get("/queue/pending")
        assert resp.status_code == 200

    def test_returns_actions_key(self, api_client):
        data = api_client.get("/queue/pending").json()
        assert "actions" in data

    def test_empty_when_gateway_unavailable(self, monkeypatch, api_client):
        """When DB is absent, /queue/pending returns an empty list (degrades gracefully)."""
        with patch("backend.webhook_server._get_queue_gateway", return_value=None):
            data = api_client.get("/queue/pending").json()
        assert data == {"actions": []}

    def test_returns_staged_actions(self, api_client_with_queue, db_path):
        """Stage a row directly, then verify /queue/pending lists it."""
        # Stage a row using the gateway directly
        from backend.queue_gateway import QueueGateway
        gw = QueueGateway()
        action_id = gw.stage(
            action_type="post_comment",
            target="PROJ-001",
            platform="github",
            workspace="test-ws",
            payload={"comment": "test"},
            confidence=0.75,
        )
        gw.close()

        # Verify the endpoint returns it
        resp = api_client_with_queue.get("/queue/pending")
        assert resp.status_code == 200
        data = resp.json()
        ids = [a["id"] for a in data["actions"]]
        assert action_id in ids

    def test_only_pending_rows_returned(self, api_client_with_queue, db_path):
        """Actions with status != 'pending' should not appear."""
        from backend.queue_gateway import QueueGateway
        gw = QueueGateway()
        pending_id = gw.stage(
            action_type="post_comment",
            target="P1",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.75,
        )
        posted_id = gw.stage(
            action_type="post_comment",
            target="P2",
            platform="github",
            workspace="ws",
            payload={},
            confidence=0.75,
        )
        gw.mark_posted(posted_id)
        gw.close()

        resp = api_client_with_queue.get("/queue/pending")
        data = resp.json()
        ids = [a["id"] for a in data["actions"]]
        assert pending_id in ids
        assert posted_id not in ids


class TestPostQueueExecuteEndpoint:
    def test_returns_400_without_action_id(self, api_client):
        resp = api_client.post("/queue/execute", json={})
        assert resp.status_code == 400

    def test_returns_404_for_unknown_action(self, api_client_with_queue):
        resp = api_client_with_queue.post("/queue/execute", json={"action_id": 99999})
        assert resp.status_code == 404

    def test_execute_marks_posted_on_success(self, api_client_with_queue, db_path):
        """A successful _execute_pm_action should result in status='posted' in the DB."""
        from backend.queue_gateway import QueueGateway
        gw = QueueGateway()
        action_id = gw.stage(
            action_type="post_comment",
            target="PROJ-001",
            platform="github",
            workspace="ws",
            payload={"comment": "test execution"},
            confidence=0.80,
        )
        gw.close()

        # Mock _execute_pm_action to succeed
        with patch(
            "backend.webhook_server.TriggerProcessor._execute_pm_action",
            return_value={"status": "posted"},
        ):
            resp = api_client_with_queue.post(
                "/queue/execute", json={"action_id": action_id}
            )

        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "posted"

        # Verify DB was updated
        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT status FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()
        assert row[0] == "posted"

    def test_execute_marks_failed_on_error(self, api_client_with_queue, db_path):
        """A failed _execute_pm_action should result in status='failed' in the DB."""
        from backend.queue_gateway import QueueGateway
        gw = QueueGateway()
        action_id = gw.stage(
            action_type="post_comment",
            target="PROJ-002",
            platform="azure",
            workspace="ws",
            payload={},
            confidence=0.60,
        )
        gw.close()

        with patch(
            "backend.webhook_server.TriggerProcessor._execute_pm_action",
            return_value={"status": "failed", "error": "connection refused"},
        ):
            resp = api_client_with_queue.post(
                "/queue/execute", json={"action_id": action_id}
            )

        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "failed"

        conn = sqlite3.connect(db_path)
        cur = conn.execute(
            "SELECT status FROM pending_actions WHERE id = ?", (action_id,)
        )
        row = cur.fetchone()
        conn.close()
        assert row[0] == "failed"
