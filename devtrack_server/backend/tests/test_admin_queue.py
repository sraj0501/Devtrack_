"""Regression coverage for the missing server-backed review surface."""
import re

import pytest
from sqlalchemy import create_engine, select
from starlette.testclient import TestClient


@pytest.fixture
def review(tmp_path, monkeypatch):
    from backend import queue_gateway as q
    from backend.admin import user_manager as users
    from backend.admin.schema import admin_metadata
    from backend.admin.app import app
    from backend.admin.auth import COOKIE_NAME, create_token

    engine = create_engine(f"sqlite:///{tmp_path / 'review.db'}")
    admin_metadata.create_all(engine)
    monkeypatch.setattr(q, "get_engine", lambda: engine)
    monkeypatch.setattr(users, "_admin_engine", engine)
    monkeypatch.setattr(users, "_schema_done", True)
    gateway = q.QueueGateway()
    action_id = gateway.stage("post_comment", "DEMO-301", "none", "test-review",
                              {"comment": "<script>alert('x')</script>"}, 0.95)
    client = TestClient(app)
    try:
        client.cookies.set(COOKIE_NAME, create_token("admin"))
        yield client, gateway, action_id, engine
    finally:
        client.close()
        engine.dispose()


def test_review_reject_and_audit(review):
    from backend.admin.schema import audit_log_table
    client, gateway, action_id, engine = review
    queue_page = client.get('/admin/queue')
    assert f'Review #{action_id}' in queue_page.text
    assert 'class="page-heading"' in queue_page.text
    queue_page = client.get('/admin/queue')
    assert 'class="page-heading"' in queue_page.text
    detail = client.get(f'/admin/queue/{action_id}')
    assert '&lt;script&gt;' in detail.text
    assert 'Destructive action.' in detail.text
    assert 'Destructive action.' in detail.text
    token = re.search(r'name="csrf" value="([^"]+)"', detail.text)[1]
    url = f'/admin/queue/{action_id}/reject'
    assert client.post(url).status_code == 403
    assert client.post(url, data={"csrf": token}).status_code == 200
    action = gateway.get_action(action_id)
    assert action['status'] == 'rejected'
    assert action['acted_by'] == 'admin' and action['acted_at']
    assert gateway.claim(action_id) is False
    assert client.post(url, data={"csrf": token}).status_code == 409
    with engine.connect() as conn:
        rows = conn.execute(select(audit_log_table)).mappings().all()
    assert len(rows) == 1
    assert rows[0]['detail'] == f'action_id={action_id}'
    assert 'queue_reject' in client.get('/admin/audit').text


def test_claim_prevents_rejection_and_replay(review):
    _, gateway, action_id, _ = review
    assert gateway.claim(action_id)
    assert not gateway.reject(action_id, 'admin')
    assert not gateway.claim(action_id)
    gateway.mark_posted(action_id)
    assert not gateway.reject(action_id, 'admin')


def test_review_requires_login(review):
    client, _, action_id, _ = review
    client.cookies.clear()
    for method, path in [('get', '/admin/queue'), ('get', f'/admin/queue/{action_id}'),
                         ('post', f'/admin/queue/{action_id}/reject')]:
        response = getattr(client, method)(path, follow_redirects=False)
        assert response.status_code == 303
        assert response.headers['location'] == '/admin/login'


def test_missing_action(review):
    client, _, _, _ = review
    assert client.get('/admin/queue/999999').status_code == 404


def test_review_token_is_bound_to_action(review):
    client, gateway, action_id, _ = review
    other_id = gateway.stage('eod_report', 'today', 'none', 'test-review', {'narrative': 'EOD'}, 0.88)
    detail = client.get(f'/admin/queue/{action_id}')
    token = re.search(r'name="csrf" value="([^"]+)"', detail.text)[1]
    assert client.post(f'/admin/queue/{other_id}/reject', data={'csrf': token}).status_code == 403
    assert gateway.get_action(other_id)['status'] == 'pending'


def test_audit_failure_rolls_back_rejection(review):
    from sqlalchemy import event
    _, gateway, action_id, engine = review

    def fail_audit(conn, cursor, statement, parameters, context, executemany):
        if statement.startswith('INSERT INTO audit_log'):
            raise RuntimeError('audit unavailable')

    event.listen(engine, 'before_cursor_execute', fail_audit)
    try:
        with pytest.raises(RuntimeError, match='audit unavailable'):
            gateway.reject(action_id, 'admin')
        assert gateway.get_action(action_id)['status'] == 'pending'
    finally:
        event.remove(engine, 'before_cursor_execute', fail_audit)
