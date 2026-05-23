"""Tests for backend.user_prompt - TUI and non-interactive behavior."""

from datetime import datetime

import pytest

from backend.user_prompt import DevTrackTUI, UserResponse


def test_non_interactive_uses_devtrack_input(monkeypatch):
    """When stdin is not TTY, DEVTRACK_INPUT env provides input."""
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    monkeypatch.setattr("sys.stdout.isatty", lambda: True)
    monkeypatch.setenv("DEVTRACK_INPUT", "Fixed bug in AB-123")

    tui = DevTrackTUI()
    assert not tui.interactive

    response = tui.prompt_user("test prompt", default="")
    assert response.raw_input == "Fixed bug in AB-123"
    assert not response.timed_out


def test_non_interactive_empty_without_env(monkeypatch):
    """When stdin is not TTY and no DEVTRACK_INPUT, returns empty/timed_out."""
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    monkeypatch.setattr("sys.stdout.isatty", lambda: True)
    monkeypatch.delenv("DEVTRACK_INPUT", raising=False)

    tui = DevTrackTUI()
    response = tui.prompt_user("test prompt", default="")
    assert response.raw_input == ""
    assert response.timed_out


def test_non_interactive_uses_default_when_no_env(monkeypatch):
    """When non-interactive, default is used when DEVTRACK_INPUT not set."""
    monkeypatch.setattr("sys.stdin.isatty", lambda: False)
    monkeypatch.setattr("sys.stdout.isatty", lambda: True)
    monkeypatch.delenv("DEVTRACK_INPUT", raising=False)

    tui = DevTrackTUI()
    response = tui.prompt_user("test prompt", default="skip")
    assert response.raw_input == "skip"
    assert response.timed_out


def test_user_response_is_valid():
    """UserResponse.is_valid checks raw_input and flags."""
    now = datetime.now()
    assert UserResponse("hello", now).is_valid()
    assert not UserResponse("", now).is_valid()
    assert not UserResponse("x", now, timed_out=True).is_valid()
    assert not UserResponse("x", now, cancelled=True).is_valid()
