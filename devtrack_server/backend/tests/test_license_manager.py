"""
Tests for backend/license_manager.py

Uses tmp_path to isolate acceptance.json reads/writes from real Data/.

Covers:
  - detect_tier / get_tier_label / check_seat_limit
  - _load_acceptance / _save_acceptance
  - is_accepted
  - ensure_accepted (env var bypass, already-accepted, decline)
  - prompt_and_accept non-interactive mode
"""
from __future__ import annotations

import json
from pathlib import Path
from unittest.mock import patch

import pytest


# ---------------------------------------------------------------------------
# Fixture: redirect license dir to tmp_path each test
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def isolated_license_dir(tmp_path, monkeypatch):
    """Override DATA_DIR so acceptance.json lands in tmp_path."""
    monkeypatch.setenv("DATA_DIR", str(tmp_path))
    # Reload so _license_dir() picks up the new DATA_DIR
    import importlib
    import backend.license_manager as lm
    importlib.reload(lm)
    yield lm


# ---------------------------------------------------------------------------
# detect_tier
# ---------------------------------------------------------------------------

class TestDetectTier:
    def test_single_user_is_personal(self, isolated_license_dir):
        lm = isolated_license_dir
        assert lm.detect_tier(1) == "personal"

    def test_zero_users_is_personal(self, isolated_license_dir):
        assert isolated_license_dir.detect_tier(0) == "personal"

    def test_two_users_is_team(self, isolated_license_dir):
        assert isolated_license_dir.detect_tier(2) == "team"

    def test_ten_users_is_team(self, isolated_license_dir):
        assert isolated_license_dir.detect_tier(10) == "team"

    def test_eleven_users_is_enterprise(self, isolated_license_dir):
        assert isolated_license_dir.detect_tier(11) == "enterprise"

    def test_large_count_is_enterprise(self, isolated_license_dir):
        assert isolated_license_dir.detect_tier(500) == "enterprise"


# ---------------------------------------------------------------------------
# get_tier_label
# ---------------------------------------------------------------------------

class TestGetTierLabel:
    def test_known_tiers_have_labels(self, isolated_license_dir):
        lm = isolated_license_dir
        assert "Personal" in lm.get_tier_label("personal")
        assert "Team" in lm.get_tier_label("team")
        assert "Enterprise" in lm.get_tier_label("enterprise")

    def test_unknown_tier_returns_tier_string(self, isolated_license_dir):
        assert isolated_license_dir.get_tier_label("unknown_tier") == "unknown_tier"


# ---------------------------------------------------------------------------
# check_seat_limit
# ---------------------------------------------------------------------------

class TestCheckSeatLimit:
    def test_personal_within_limit(self, isolated_license_dir):
        ok, msg = isolated_license_dir.check_seat_limit(1)
        assert ok is True
        assert "OK" in msg

    def test_team_within_limit(self, isolated_license_dir):
        ok, msg = isolated_license_dir.check_seat_limit(10)
        assert ok is True

    def test_enterprise_exceeds_limit(self, isolated_license_dir):
        ok, msg = isolated_license_dir.check_seat_limit(11)
        assert ok is False
        assert "commercial licence" in msg.lower() or "licence" in msg.lower()

    def test_message_includes_user_count_when_over(self, isolated_license_dir):
        _, msg = isolated_license_dir.check_seat_limit(25)
        assert "25" in msg


# ---------------------------------------------------------------------------
# Acceptance record: _save_acceptance / _load_acceptance / is_accepted
# ---------------------------------------------------------------------------

class TestAcceptanceRecord:
    def test_not_accepted_when_no_file(self, isolated_license_dir):
        assert isolated_license_dir.is_accepted() is False

    def test_save_creates_file(self, isolated_license_dir, tmp_path):
        isolated_license_dir._save_acceptance("testuser")
        path = tmp_path / "license" / "acceptance.json"
        assert path.exists()

    def test_saved_record_has_required_fields(self, isolated_license_dir, tmp_path):
        isolated_license_dir._save_acceptance("testuser")
        path = tmp_path / "license" / "acceptance.json"
        data = json.loads(path.read_text())
        assert "terms_version" in data
        assert "accepted_at" in data
        assert data["user_identifier"] == "testuser"

    def test_is_accepted_after_save(self, isolated_license_dir):
        isolated_license_dir._save_acceptance()
        assert isolated_license_dir.is_accepted() is True

    def test_load_returns_none_for_corrupt_file(self, isolated_license_dir, tmp_path):
        path = tmp_path / "license" / "acceptance.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("not valid json{{{")
        assert isolated_license_dir._load_acceptance() is None

    def test_load_returns_none_for_missing_fields(self, isolated_license_dir, tmp_path):
        path = tmp_path / "license" / "acceptance.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text('{"terms_version": "1.0"}')  # missing accepted_at
        assert isolated_license_dir._load_acceptance() is None

    def test_version_mismatch_not_accepted(self, isolated_license_dir, tmp_path):
        path = tmp_path / "license" / "acceptance.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text('{"terms_version": "0.9", "accepted_at": "2025-01-01T00:00:00+00:00"}')
        assert isolated_license_dir.is_accepted() is False

    def test_get_acceptance_record_returns_dict_when_accepted(self, isolated_license_dir):
        isolated_license_dir._save_acceptance("dev")
        record = isolated_license_dir.get_acceptance_record()
        assert isinstance(record, dict)
        assert record["user_identifier"] == "dev"

    def test_get_acceptance_record_returns_none_when_not_accepted(self, isolated_license_dir):
        assert isolated_license_dir.get_acceptance_record() is None


# ---------------------------------------------------------------------------
# ensure_accepted
# ---------------------------------------------------------------------------

class TestEnsureAccepted:
    def test_env_var_bypass_auto_accepts(self, isolated_license_dir, monkeypatch):
        monkeypatch.setenv("DEVTRACK_AUTO_ACCEPT_TERMS", "1")
        result = isolated_license_dir.ensure_accepted()
        assert result is True
        assert isolated_license_dir.is_accepted() is True

    def test_env_var_bypass_idempotent(self, isolated_license_dir, monkeypatch):
        monkeypatch.setenv("DEVTRACK_AUTO_ACCEPT_TERMS", "1")
        isolated_license_dir.ensure_accepted()
        result = isolated_license_dir.ensure_accepted()
        assert result is True

    def test_already_accepted_returns_true_without_prompt(self, isolated_license_dir):
        isolated_license_dir._save_acceptance()
        # No monkeypatching of input — should return True without asking
        result = isolated_license_dir.ensure_accepted()
        assert result is True

    def test_non_interactive_auto_accepts(self, isolated_license_dir, capsys):
        result = isolated_license_dir.ensure_accepted(non_interactive=True)
        assert result is True
        assert isolated_license_dir.is_accepted() is True


# ---------------------------------------------------------------------------
# prompt_and_accept non-interactive
# ---------------------------------------------------------------------------

class TestPromptAndAccept:
    def test_non_interactive_accepts_and_returns_true(self, isolated_license_dir, capsys):
        result = isolated_license_dir.prompt_and_accept(non_interactive=True)
        assert result is True
        assert isolated_license_dir.is_accepted() is True

    def test_interactive_yes_accepts(self, isolated_license_dir):
        with patch("builtins.input", return_value="yes"):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is True
        assert isolated_license_dir.is_accepted() is True

    def test_interactive_y_accepts(self, isolated_license_dir):
        with patch("builtins.input", return_value="y"):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is True

    def test_interactive_no_declines(self, isolated_license_dir):
        with patch("builtins.input", return_value="no"):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is False
        assert isolated_license_dir.is_accepted() is False

    def test_interactive_n_declines(self, isolated_license_dir):
        with patch("builtins.input", return_value="n"):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is False

    def test_interactive_eof_declines(self, isolated_license_dir):
        with patch("builtins.input", side_effect=EOFError):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is False

    def test_interactive_invalid_then_yes(self, isolated_license_dir):
        responses = iter(["maybe", "sure", "yes"])
        with patch("builtins.input", side_effect=responses):
            result = isolated_license_dir.prompt_and_accept(non_interactive=False)
        assert result is True
