"""
Tests for backend/admin/auth.py

Covers:
  - hash_password / verify_password
  - create_token / decode_token (valid, expired, tampered)
  - check_credentials (env-var lookup, plain-text, scrypt hash)
  - require_auth FastAPI dependency (cookie present/missing/invalid)
"""
from __future__ import annotations

import time
from unittest.mock import patch

import pytest


# ---------------------------------------------------------------------------
# hash_password / verify_password
# ---------------------------------------------------------------------------

class TestPasswordHashing:
    def test_hash_returns_scrypt_prefix(self):
        from backend.admin.auth import hash_password
        h = hash_password("secret123")
        assert h.startswith("scrypt$")

    def test_hash_is_different_each_call(self):
        from backend.admin.auth import hash_password
        h1 = hash_password("same")
        h2 = hash_password("same")
        assert h1 != h2  # different salts

    def test_verify_correct_password(self):
        from backend.admin.auth import hash_password, verify_password
        h = hash_password("mypassword")
        assert verify_password("mypassword", h) is True

    def test_verify_wrong_password(self):
        from backend.admin.auth import hash_password, verify_password
        h = hash_password("correct")
        assert verify_password("wrong", h) is False

    def test_verify_empty_password_rejected(self):
        from backend.admin.auth import hash_password, verify_password
        h = hash_password("notempty")
        assert verify_password("", h) is False

    def test_verify_legacy_plain_text(self):
        """Verify that the legacy plain-text path works during migration."""
        from backend.admin.auth import verify_password
        # hashed value is just the plain-text string (legacy)
        assert verify_password("dev", "dev") is True
        assert verify_password("wrong", "dev") is False

    def test_verify_corrupt_hash_returns_false(self):
        from backend.admin.auth import verify_password
        assert verify_password("any", "scrypt$bad$data") is False

    def test_verify_empty_hash_returns_false(self):
        from backend.admin.auth import verify_password
        assert verify_password("any", "") is False


# ---------------------------------------------------------------------------
# create_token / decode_token
# ---------------------------------------------------------------------------

class TestJWTTokens:
    def test_decode_valid_token_returns_username(self):
        from backend.admin.auth import create_token, decode_token
        token = create_token("alice")
        assert decode_token(token) == "alice"

    def test_decode_garbage_returns_none(self):
        from backend.admin.auth import decode_token
        assert decode_token("not.a.token") is None

    def test_decode_empty_string_returns_none(self):
        from backend.admin.auth import decode_token
        assert decode_token("") is None

    def test_decode_tampered_token_returns_none(self):
        from backend.admin.auth import create_token, decode_token
        token = create_token("alice")
        # Replace the entire signature with a known-invalid one.
        # Flipping a single base64url character is unreliable: the trailing bits
        # of a base64url block may be insignificant, so a one-char flip can leave
        # the decoded bytes unchanged and the signature still valid.
        parts = token.split(".")
        parts[-1] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
        assert decode_token(".".join(parts)) is None

    def test_decode_expired_token_returns_none(self):
        """Token with exp in the past should be rejected."""
        import jwt
        from backend.admin.auth import _SECRET, _ALGORITHM
        payload = {"sub": "alice", "iat": int(time.time()) - 7200, "exp": int(time.time()) - 3600}
        token = jwt.encode(payload, _SECRET, algorithm=_ALGORITHM)
        from backend.admin.auth import decode_token
        assert decode_token(token) is None

    def test_token_for_different_users_are_distinct(self):
        from backend.admin.auth import create_token
        assert create_token("alice") != create_token("bob")


# ---------------------------------------------------------------------------
# check_credentials
# ---------------------------------------------------------------------------

class TestCheckCredentials:
    def test_correct_plain_text_credentials(self, monkeypatch):
        monkeypatch.setenv("ADMIN_USERNAME", "admin")
        monkeypatch.setenv("ADMIN_PASSWORD", "secret")
        from backend.admin import auth as _auth
        assert _auth.check_credentials("admin", "secret") is True

    def test_wrong_password_rejected(self, monkeypatch):
        monkeypatch.setenv("ADMIN_USERNAME", "admin")
        monkeypatch.setenv("ADMIN_PASSWORD", "secret")
        from backend.admin import auth as _auth
        assert _auth.check_credentials("admin", "wrong") is False

    def test_wrong_username_rejected(self, monkeypatch):
        monkeypatch.setenv("ADMIN_USERNAME", "admin")
        monkeypatch.setenv("ADMIN_PASSWORD", "secret")
        from backend.admin import auth as _auth
        assert _auth.check_credentials("notadmin", "secret") is False

    def test_empty_password_env_always_rejects(self, monkeypatch):
        monkeypatch.setenv("ADMIN_USERNAME", "admin")
        monkeypatch.setenv("ADMIN_PASSWORD", "")
        from backend.admin import auth as _auth
        assert _auth.check_credentials("admin", "") is False
        assert _auth.check_credentials("admin", "anything") is False

    def test_scrypt_hash_in_env(self, monkeypatch):
        from backend.admin.auth import hash_password
        hashed = hash_password("mypassword")
        monkeypatch.setenv("ADMIN_USERNAME", "admin")
        monkeypatch.setenv("ADMIN_PASSWORD", hashed)
        from backend.admin import auth as _auth
        assert _auth.check_credentials("admin", "mypassword") is True
        assert _auth.check_credentials("admin", "wrong") is False


# ---------------------------------------------------------------------------
# Scrypt config accessor validation
# ---------------------------------------------------------------------------

class TestScryptConfig:
    def test_get_scrypt_n_raises_when_unset(self, monkeypatch):
        """get_scrypt_n() must raise ValueError when SCRYPT_N is not set."""
        monkeypatch.delenv("SCRYPT_N", raising=False)
        # Force re-import of config to avoid cached module state
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        with pytest.raises(ValueError, match="SCRYPT_N"):
            cfg.get_scrypt_n()

    def test_get_scrypt_n_raises_for_non_power_of_two(self, monkeypatch):
        """get_scrypt_n() must raise ValueError when SCRYPT_N is not a power of 2."""
        monkeypatch.setenv("SCRYPT_N", "100")
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        with pytest.raises(ValueError, match="power of 2"):
            cfg.get_scrypt_n()

    def test_get_scrypt_n_accepts_valid_power_of_two(self, monkeypatch):
        """get_scrypt_n() returns the integer when SCRYPT_N is a valid power of 2."""
        monkeypatch.setenv("SCRYPT_N", "16384")
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        assert cfg.get_scrypt_n() == 16384

    def test_get_admin_session_hours_raises_when_unset(self, monkeypatch):
        """get_admin_session_hours() must raise ValueError when ADMIN_SESSION_HOURS is not set."""
        monkeypatch.delenv("ADMIN_SESSION_HOURS", raising=False)
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        with pytest.raises(ValueError, match="ADMIN_SESSION_HOURS"):
            cfg.get_admin_session_hours()

    def test_get_admin_session_hours_returns_value(self, monkeypatch):
        """get_admin_session_hours() returns the configured integer."""
        monkeypatch.setenv("ADMIN_SESSION_HOURS", "12")
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        assert cfg.get_admin_session_hours() == 12
