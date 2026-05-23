"""
Admin Console authentication — HTTP Basic login → JWT session cookie.

Sessions are stateless (validated via JWT signature); no server-side session store.
Config:
    ADMIN_USERNAME   — admin account username   (default: admin)
    ADMIN_PASSWORD   — plain-text password checked against stored hash
    ADMIN_SECRET_KEY — secret for JWT signing   (auto-generated if absent)
    ADMIN_SESSION_HOURS — session lifetime hours (default: 8)
"""
from __future__ import annotations

import os
import secrets
import time
from typing import Optional

import hashlib
import hmac

import jwt
from fastapi import Cookie, HTTPException, Request, status

# Read config once at import time — all values required to be set, or we use
# safe defaults suitable for a self-hosted local install.
def _cfg(key: str, default: str = "") -> str:
    from backend.config import get
    return get(key, default)


def _secret_key() -> str:
    key = _cfg("ADMIN_SECRET_KEY")
    if not key:
        # Warn once; the key will be ephemeral (sessions die on restart)
        key = secrets.token_hex(32)
    return key


from backend.config import (
    get_scrypt_n,
    get_scrypt_r,
    get_scrypt_p,
    get_scrypt_dklen,
)

_SECRET = _secret_key()
_ALGORITHM = "HS256"
_SESSION_HOURS = int(_cfg("ADMIN_SESSION_HOURS", "8"))
COOKIE_NAME = "devtrack_admin_session"

_SCRYPT_N    = get_scrypt_n()
_SCRYPT_R    = get_scrypt_r()
_SCRYPT_P    = get_scrypt_p()
_SCRYPT_DKLEN = get_scrypt_dklen()


# ---------------------------------------------------------------------------
# Password helpers
# ---------------------------------------------------------------------------

def hash_password(plain: str) -> str:
    """Hash password with scrypt (stdlib, no external deps)."""
    salt = secrets.token_hex(16)
    key = hashlib.scrypt(plain.encode(), salt=salt.encode(),
                         n=_SCRYPT_N, r=_SCRYPT_R, p=_SCRYPT_P, dklen=_SCRYPT_DKLEN)
    return f"scrypt${salt}${key.hex()}"


def verify_password(plain: str, hashed: str) -> bool:
    try:
        if hashed.startswith("scrypt$"):
            _, salt, stored_hex = hashed.split("$", 2)
            key = hashlib.scrypt(plain.encode(), salt=salt.encode(),
                                 n=_SCRYPT_N, r=_SCRYPT_R, p=_SCRYPT_P, dklen=_SCRYPT_DKLEN)
            return hmac.compare_digest(key.hex(), stored_hex)
        # Legacy plain-text check (migration path)
        return hmac.compare_digest(plain, hashed)
    except Exception:
        return False


# ---------------------------------------------------------------------------
# Token helpers
# ---------------------------------------------------------------------------

def create_token(username: str) -> str:
    payload = {
        "sub": username,
        "iat": int(time.time()),
        "exp": int(time.time()) + _SESSION_HOURS * 3600,
    }
    return jwt.encode(payload, _SECRET, algorithm=_ALGORITHM)


def decode_token(token: str) -> Optional[str]:
    """Return username from token, or None if invalid/expired."""
    try:
        payload = jwt.decode(token, _SECRET, algorithms=[_ALGORITHM])
        return payload.get("sub")
    except jwt.PyJWTError:
        return None


# ---------------------------------------------------------------------------
# Dependency: require authenticated session
# ---------------------------------------------------------------------------

def require_auth(
    request: Request,
    devtrack_admin_session: Optional[str] = Cookie(default=None),
) -> str:
    """FastAPI dependency — returns username or redirects to /admin/login."""
    token = devtrack_admin_session
    if not token:
        raise HTTPException(
            status_code=status.HTTP_303_SEE_OTHER,
            headers={"Location": "/admin/login"},
        )
    username = decode_token(token)
    if not username:
        raise HTTPException(
            status_code=status.HTTP_303_SEE_OTHER,
            headers={"Location": "/admin/login"},
        )
    return username


# ---------------------------------------------------------------------------
# Credential check against .env
# ---------------------------------------------------------------------------

def check_credentials(username: str, password: str) -> bool:
    """
    Validates username+password.

    Uses ADMIN_USERNAME / ADMIN_PASSWORD from .env.
    ADMIN_PASSWORD may be a bcrypt hash (starts with $2b$) or plain text
    (plain text is compared directly — only suitable for local dev).
    """
    expected_user = _cfg("ADMIN_USERNAME", "admin")
    if username != expected_user:
        return False
    stored = _cfg("ADMIN_PASSWORD", "")
    if not stored:
        return False
    if stored.startswith("scrypt$") or stored.startswith("$2b$") or stored.startswith("$2a$"):
        return verify_password(password, stored)
    # Plain-text fallback (local dev only) — use constant-time compare
    return hmac.compare_digest(password, stored)
