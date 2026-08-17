"""
User manager — CRUD for admin console users.

Schema
------
admin_users    (id, username, password_hash, role, created_at, last_login, disabled)
admin_api_keys (id, user_id, key_prefix, key_hash, label, created_at, last_used)
audit_log      (id, username, action, detail, ip, ts)

Backend selection
-----------------
PostgreSQL mode (POSTGRES_URL set)  → tables live in the main PostgreSQL DB.
SQLite mode                         → separate admin.db at DATABASE_DIR/admin.db
                                      (backwards-compatible with existing deployments).
"""
from __future__ import annotations

import hashlib
import secrets
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

from sqlalchemy import select
from sqlalchemy.engine import Engine

from backend.admin.auth import hash_password, verify_password
from backend.admin.schema import (
    admin_api_keys_table,
    admin_metadata as _admin_metadata,
    admin_users_table,
    audit_log_table,
    connected_clients_table,
)

# ---------------------------------------------------------------------------
# Engine: shared PostgreSQL or separate admin.db
# ---------------------------------------------------------------------------

_admin_engine: Optional[Engine] = None
_schema_done: bool = False


def _get_admin_engine() -> Engine:
    global _admin_engine
    if _admin_engine is not None:
        return _admin_engine

    from backend.config import postgres_url
    if postgres_url():
        # In PostgreSQL mode: admin tables live in the same main DB.
        from backend.db.engine import get_engine
        _admin_engine = get_engine()
    else:
        # In SQLite mode: keep separate admin.db for backwards compatibility.
        from sqlalchemy import create_engine, event as sa_event
        from backend.config import get_database_dir, get_project_root
        db_dir = get_database_dir() or str(Path(get_project_root() or ".") / "Data" / "db")
        admin_path = Path(db_dir) / "admin.db"
        admin_path.parent.mkdir(parents=True, exist_ok=True)
        _admin_engine = create_engine(
            f"sqlite:///{admin_path}",
            connect_args={"check_same_thread": False, "timeout": 30},
        )

        @sa_event.listens_for(_admin_engine, "connect")
        def _pragmas(conn, _record):
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute("PRAGMA foreign_keys=ON")

    return _admin_engine


def _init() -> Engine:
    global _schema_done
    eng = _get_admin_engine()
    if not _schema_done:
        if eng.dialect.name == "postgresql":
            from backend.db.engine import ensure_tables

            ensure_tables(eng, schema_metadata=_admin_metadata)
        else:
            _admin_metadata.create_all(eng)
        _schema_done = True
    return eng


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


# ---------------------------------------------------------------------------
# init_db — kept for call-site compatibility
# ---------------------------------------------------------------------------

def init_db() -> None:
    _init()


# ---------------------------------------------------------------------------
# User dataclasses
# ---------------------------------------------------------------------------

@dataclass
class AdminUser:
    id: int
    username: str
    role: str
    created_at: str
    last_login: Optional[str]
    disabled: bool = False


@dataclass
class ApiKey:
    id: int
    user_id: int
    key_prefix: str
    label: str
    created_at: str
    last_used: Optional[str]


# ---------------------------------------------------------------------------
# User CRUD
# ---------------------------------------------------------------------------

def _row_to_user(row) -> AdminUser:
    d = dict(row)
    return AdminUser(
        id=d["id"],
        username=d["username"],
        role=d["role"],
        created_at=d["created_at"],
        last_login=d.get("last_login"),
        disabled=bool(d.get("disabled", 0)),
    )


def list_users() -> list[AdminUser]:
    eng = _init()
    with eng.connect() as conn:
        rows = conn.execute(
            select(admin_users_table).order_by(admin_users_table.c.id)
        ).mappings().all()
    return [_row_to_user(r) for r in rows]


def get_user(username: str) -> Optional[AdminUser]:
    eng = _init()
    with eng.connect() as conn:
        row = conn.execute(
            select(admin_users_table).where(admin_users_table.c.username == username)
        ).mappings().first()
    return _row_to_user(row) if row else None


def create_user(username: str, password: str, role: str = "viewer") -> AdminUser:
    hashed = hash_password(password)
    now = _now()
    eng = _init()
    with eng.begin() as conn:
        conn.execute(
            admin_users_table.insert().values(
                username=username,
                password_hash=hashed,
                role=role,
                created_at=now,
                disabled=0,
            )
        )
    return get_user(username)  # type: ignore[return-value]


def update_password(username: str, new_password: str) -> bool:
    hashed = hash_password(new_password)
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_users_table.update()
            .where(admin_users_table.c.username == username)
            .values(password_hash=hashed)
        )
    return result.rowcount > 0


def update_role(username: str, role: str) -> bool:
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_users_table.update()
            .where(admin_users_table.c.username == username)
            .values(role=role)
        )
    return result.rowcount > 0


def delete_user(username: str) -> bool:
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_users_table.delete().where(admin_users_table.c.username == username)
        )
    return result.rowcount > 0


def disable_user(username: str) -> bool:
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_users_table.update()
            .where(admin_users_table.c.username == username)
            .values(disabled=1)
        )
    return result.rowcount > 0


def enable_user(username: str) -> bool:
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_users_table.update()
            .where(admin_users_table.c.username == username)
            .values(disabled=0)
        )
    return result.rowcount > 0


def touch_last_login(username: str) -> None:
    eng = _init()
    with eng.begin() as conn:
        conn.execute(
            admin_users_table.update()
            .where(admin_users_table.c.username == username)
            .values(last_login=_now())
        )


def verify_user_password(username: str, password: str) -> bool:
    eng = _init()
    with eng.connect() as conn:
        row = conn.execute(
            select(admin_users_table.c.password_hash).where(
                admin_users_table.c.username == username
            )
        ).first()
    if not row:
        return False
    return verify_password(password, row.password_hash)


def ensure_default_admin(username: str, password: str) -> None:
    """Create the initial admin account if no users exist yet."""
    eng = _init()
    with eng.connect() as conn:
        count = conn.execute(
            select(admin_users_table.c.id).limit(1)
        ).first()
    if not count:
        create_user(username, password, role="admin")


# ---------------------------------------------------------------------------
# API key management
# ---------------------------------------------------------------------------

def _row_to_api_key(row) -> ApiKey:
    d = dict(row)
    return ApiKey(
        id=d["id"],
        user_id=d["user_id"],
        key_prefix=d["key_prefix"],
        label=d["label"],
        created_at=d["created_at"],
        last_used=d.get("last_used"),
    )


def create_api_key(username: str, label: str = "") -> tuple[str, ApiKey]:
    """Returns (raw_key, ApiKey) — raw_key is shown once only."""
    user = get_user(username)
    if not user:
        raise ValueError(f"User not found: {username}")
    raw = secrets.token_urlsafe(32)
    prefix = raw[:8]
    key_hash = hashlib.sha256(raw.encode()).hexdigest()
    now = _now()
    eng = _init()
    with eng.begin() as conn:
        conn.execute(
            admin_api_keys_table.insert().values(
                user_id=user.id,
                key_prefix=prefix,
                key_hash=key_hash,
                label=label,
                created_at=now,
            )
        )
    with eng.connect() as conn:
        row = conn.execute(
            select(admin_api_keys_table)
            .where(admin_api_keys_table.c.user_id == user.id)
            .order_by(admin_api_keys_table.c.id.desc())
            .limit(1)
        ).mappings().first()
    return raw, _row_to_api_key(row)  # type: ignore[arg-type]


def list_api_keys(username: str) -> list[ApiKey]:
    user = get_user(username)
    if not user:
        return []
    eng = _init()
    with eng.connect() as conn:
        rows = conn.execute(
            select(admin_api_keys_table)
            .where(admin_api_keys_table.c.user_id == user.id)
            .order_by(admin_api_keys_table.c.id)
        ).mappings().all()
    return [_row_to_api_key(r) for r in rows]


def revoke_api_key(key_id: int) -> bool:
    eng = _init()
    with eng.begin() as conn:
        result = conn.execute(
            admin_api_keys_table.delete().where(admin_api_keys_table.c.id == key_id)
        )
    return result.rowcount > 0


# ---------------------------------------------------------------------------
# Audit log
# ---------------------------------------------------------------------------

def log_action(username: str, action: str, detail: str = "", ip: str = "") -> None:
    eng = _init()
    with eng.begin() as conn:
        conn.execute(
            audit_log_table.insert().values(
                username=username,
                action=action,
                detail=detail,
                ip=ip,
                ts=_now(),
            )
        )


def get_audit_log(limit: int | None = None) -> list[dict]:
    if limit is None:
        from backend.config import get_audit_log_limit
        limit = get_audit_log_limit()
    eng = _init()
    with eng.connect() as conn:
        rows = conn.execute(
            select(audit_log_table)
            .order_by(audit_log_table.c.id.desc())
            .limit(limit)
        ).mappings().all()
    return [dict(r) for r in rows]


# ---------------------------------------------------------------------------
# Connected clients (heartbeat registry)
# ---------------------------------------------------------------------------

import json as _json
from datetime import datetime as _dt, timezone as _tz, timedelta as _td


def upsert_client(client_id: str, version: str, tls_enabled: bool,
                  workspaces: list[dict], ip: str) -> None:
    eng = _init()
    now = _dt.now(_tz.utc).isoformat()
    ws_json = _json.dumps(workspaces)
    tls_int = 1 if tls_enabled else 0
    with eng.begin() as conn:
        existing = conn.execute(
            select(connected_clients_table.c.id)
            .where(connected_clients_table.c.client_id == client_id)
        ).first()
        if existing:
            conn.execute(
                connected_clients_table.update()
                .where(connected_clients_table.c.client_id == client_id)
                .values(version=version, tls_enabled=tls_int,
                        workspaces=ws_json, last_seen=now, ip=ip)
            )
        else:
            conn.execute(
                connected_clients_table.insert().values(
                    client_id=client_id, version=version, tls_enabled=tls_int,
                    workspaces=ws_json, last_seen=now, ip=ip,
                )
            )


def list_clients(stale_minutes: int = 5) -> list[dict]:
    """Return clients seen within stale_minutes; mark each with is_active."""
    eng = _init()
    cutoff = (_dt.now(_tz.utc) - _td(minutes=stale_minutes)).isoformat()
    with eng.connect() as conn:
        rows = conn.execute(
            select(connected_clients_table)
            .order_by(connected_clients_table.c.last_seen.desc())
        ).mappings().all()
    result = []
    for r in rows:
        d = dict(r)
        d["workspaces"] = _json.loads(d.get("workspaces") or "[]")
        d["tls_enabled"] = bool(d.get("tls_enabled", 0))
        d["is_active"] = d["last_seen"] >= cutoff
        result.append(d)
    return result


def prune_stale_clients(stale_minutes: int = 10) -> int:
    """Delete clients not seen within stale_minutes. Returns count removed."""
    eng = _init()
    cutoff = (_dt.now(_tz.utc) - _td(minutes=stale_minutes)).isoformat()
    with eng.begin() as conn:
        result = conn.execute(
            connected_clients_table.delete()
            .where(connected_clients_table.c.last_seen < cutoff)
        )
    return result.rowcount
