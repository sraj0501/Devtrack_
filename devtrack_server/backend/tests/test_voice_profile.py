"""
Tests for voice_profile.py — ProfileGenerator class and POST /voice/profile/generate endpoint.

Acceptance criteria verified:
- LLM available (mocked): generate() returns non-empty string starting with '#'
- LLM unavailable (raises): generate() returns fallback template, no exception propagates
- ChromaDB empty: generate() returns fallback template immediately, LLM not called
- save() writes to correct path; creates directory if absent; returns correct Path
- Endpoint smoke test: POST /voice/profile/generate returns {"path": ..., "word_count": N}
"""
from __future__ import annotations

import pathlib
from unittest.mock import MagicMock, patch

import pytest

# ── ProfileGenerator unit tests ───────────────────────────────────────────────


def test_generate_llm_available_returns_profile_heading():
    """When LLM returns a response, generate() returns a string starting with
    '# Developer Voice Profile'."""
    from backend.voice_profile import ProfileGenerator

    mock_provider = MagicMock()
    mock_provider.generate.return_value = (
        "# Developer Voice Profile\n\n## Formality Level\nInformal, direct."
    )

    gen = ProfileGenerator(provider=mock_provider)

    # Patch _retrieve_commit_messages to return 5 fake messages so we don't need ChromaDB.
    with patch.object(
        gen,
        "_retrieve_commit_messages",
        return_value=[
            "fix auth token expiry bug",
            "add retry logic to HTTP client",
            "refactor config loader",
            "remove unused imports",
            "update README with setup steps",
        ],
    ):
        result = gen.generate(["some/repo"])

    assert result.startswith("# Developer Voice Profile"), (
        f"Expected '# Developer Voice Profile' at start, got: {result[:80]!r}"
    )
    assert len(result) > 30


def test_generate_llm_unavailable_returns_fallback():
    """When LLM provider raises an exception, generate() returns the fallback
    template and does NOT raise."""
    from backend.voice_profile import ProfileGenerator, _FALLBACK_TEMPLATE

    mock_provider = MagicMock()
    mock_provider.generate.side_effect = RuntimeError("Ollama is down")

    gen = ProfileGenerator(provider=mock_provider)

    with patch.object(
        gen,
        "_retrieve_commit_messages",
        return_value=["fix bug", "add feature", "update docs"],
    ):
        result = gen.generate(["some/repo"])

    assert result == _FALLBACK_TEMPLATE, (
        f"Expected fallback template, got: {result!r}"
    )


def test_generate_llm_returns_none_returns_fallback():
    """When LLM provider returns None (model unavailable), generate() returns fallback."""
    from backend.voice_profile import ProfileGenerator, _FALLBACK_TEMPLATE

    mock_provider = MagicMock()
    mock_provider.generate.return_value = None

    gen = ProfileGenerator(provider=mock_provider)

    with patch.object(
        gen,
        "_retrieve_commit_messages",
        return_value=["fix bug", "add feature"],
    ):
        result = gen.generate(["some/repo"])

    assert result == _FALLBACK_TEMPLATE


def test_generate_chromadb_empty_returns_fallback_llm_not_called():
    """When ChromaDB has no entries for the given repo_paths, generate() returns
    the fallback template immediately — the LLM is never invoked."""
    from backend.voice_profile import ProfileGenerator, _FALLBACK_TEMPLATE

    mock_provider = MagicMock()
    gen = ProfileGenerator(provider=mock_provider)

    with patch.object(gen, "_retrieve_commit_messages", return_value=[]):
        result = gen.generate(["some/repo"])

    assert result == _FALLBACK_TEMPLATE, (
        f"Expected fallback when ChromaDB empty, got: {result!r}"
    )
    mock_provider.generate.assert_not_called()


def test_generate_never_raises_on_unexpected_exception():
    """generate() wraps all internal logic in try/except — never raises."""
    from backend.voice_profile import ProfileGenerator, _FALLBACK_TEMPLATE

    gen = ProfileGenerator(provider=None)

    # Force an exception inside the internal method
    with patch.object(gen, "_generate", side_effect=ValueError("Catastrophic error")):
        result = gen.generate(["repo"])

    assert result == _FALLBACK_TEMPLATE


def test_generate_heading_prepended_when_llm_omits_it():
    """If LLM returns a response without the required heading, generate() prepends it."""
    from backend.voice_profile import ProfileGenerator

    mock_provider = MagicMock()
    # LLM response missing the heading
    mock_provider.generate.return_value = "Informal style. Uses imperative verbs."

    gen = ProfileGenerator(provider=mock_provider)

    with patch.object(
        gen,
        "_retrieve_commit_messages",
        return_value=["fix bug", "add feature", "remove old code"],
    ):
        result = gen.generate(["some/repo"])

    assert result.startswith("# Developer Voice Profile")


# ── save() tests ──────────────────────────────────────────────────────────────


def test_save_creates_directory_and_writes_file(tmp_path):
    """save() creates {data_dir}/learning/ if absent and writes the profile."""
    from backend.voice_profile import ProfileGenerator

    gen = ProfileGenerator()
    profile_text = "# Developer Voice Profile\n\nTest content."

    # data_dir does not have a learning/ sub-dir yet
    data_dir = tmp_path / "data"  # does not exist yet
    data_dir.mkdir()

    result_path = gen.save(profile_text, str(data_dir))

    expected = data_dir / "learning" / "profile.md"
    assert result_path == expected, f"Expected {expected}, got {result_path}"
    assert expected.exists(), "profile.md was not created"
    assert expected.read_text(encoding="utf-8") == profile_text


def test_save_overwrites_existing_file(tmp_path):
    """save() overwrites an existing profile.md (re-generate always wins)."""
    from backend.voice_profile import ProfileGenerator

    gen = ProfileGenerator()
    learning_dir = tmp_path / "learning"
    learning_dir.mkdir()
    profile_file = learning_dir / "profile.md"
    profile_file.write_text("# Developer Voice Profile\n\nOld content.", encoding="utf-8")

    new_text = "# Developer Voice Profile\n\nNew updated content."
    result_path = gen.save(new_text, str(tmp_path))

    assert result_path.read_text(encoding="utf-8") == new_text


def test_save_returns_path_object(tmp_path):
    """save() returns a pathlib.Path."""
    from backend.voice_profile import ProfileGenerator

    gen = ProfileGenerator()
    result = gen.save("# Developer Voice Profile\n\nContent.", str(tmp_path))
    assert isinstance(result, pathlib.Path)


# ── Endpoint smoke tests ──────────────────────────────────────────────────────


def test_endpoint_voice_profile_generate_returns_path_and_word_count(tmp_path, monkeypatch):
    """POST /voice/profile/generate returns {"path": "...", "word_count": N}."""
    import os

    # Point DATA_DIR at tmp_path so no real filesystem side effects
    monkeypatch.setenv("DATA_DIR", str(tmp_path))

    from fastapi.testclient import TestClient

    # Import the app after setting env so config picks up DATA_DIR
    from backend.webhook_server import app

    client = TestClient(app, raise_server_exceptions=True)

    profile_text = "# Developer Voice Profile\n\nThis is a test profile with several words."

    with (
        patch("backend.voice_profile.ProfileGenerator.generate", return_value=profile_text),
        patch(
            "backend.voice_profile.ProfileGenerator.save",
            return_value=tmp_path / "learning" / "profile.md",
        ),
    ):
        response = client.post(
            "/voice/profile/generate",
            json={"repo_paths": ["/some/repo"]},
            headers={"X-DevTrack-API-Key": os.environ.get("DEVTRACK_API_KEY", "")},
        )

    assert response.status_code == 200, f"Expected 200, got {response.status_code}: {response.text}"
    body = response.json()
    assert "path" in body, f"Missing 'path' in response: {body}"
    assert "word_count" in body, f"Missing 'word_count' in response: {body}"
    assert isinstance(body["word_count"], int)
    assert body["word_count"] > 0


def test_endpoint_voice_profile_generate_empty_body(tmp_path, monkeypatch):
    """POST /voice/profile/generate with empty body still returns valid response."""
    import os

    monkeypatch.setenv("DATA_DIR", str(tmp_path))

    from fastapi.testclient import TestClient
    from backend.webhook_server import app

    client = TestClient(app, raise_server_exceptions=True)

    profile_text = "# Developer Voice Profile\n\nFallback content here."

    with (
        patch("backend.voice_profile.ProfileGenerator.generate", return_value=profile_text),
        patch(
            "backend.voice_profile.ProfileGenerator.save",
            return_value=tmp_path / "learning" / "profile.md",
        ),
    ):
        response = client.post(
            "/voice/profile/generate",
            json={},
            headers={"X-DevTrack-API-Key": os.environ.get("DEVTRACK_API_KEY", "")},
        )

    assert response.status_code == 200
    body = response.json()
    assert "word_count" in body


# ── PersonalizedAI.get_style_instruction() path tests ────────────────────────


def test_get_style_instruction_reads_profile_md(tmp_path, monkeypatch):
    """get_style_instruction() reads {DATA_DIR}/learning/profile.md via config
    when the file exists and contains non-fallback content."""
    monkeypatch.setenv("DATA_DIR", str(tmp_path))

    # Create a profile.md with real content
    learning_dir = tmp_path / "learning"
    learning_dir.mkdir()
    profile_text = (
        "# Developer Voice Profile\n\n## Formality Level\nInformal and direct.\n\n"
        "## Verb Mood\nImperative (fix, add, remove)."
    )
    (learning_dir / "profile.md").write_text(profile_text, encoding="utf-8")

    from backend.personalized_ai import PersonalizedAI

    ai = PersonalizedAI.__new__(PersonalizedAI)
    ai.profile = None  # no SQLite profile loaded
    ai.user_email = "test@test.com"

    result = ai.get_style_instruction("commit")

    assert result.startswith("[STYLE:"), f"Expected [STYLE: ...], got {result!r}"
    assert "Informal" in result or "Developer Voice Profile" in result


def test_get_style_instruction_fallback_when_profile_md_missing(tmp_path, monkeypatch):
    """get_style_instruction() returns '' when no profile.md and no SQLite profile."""
    monkeypatch.setenv("DATA_DIR", str(tmp_path))

    from backend.personalized_ai import PersonalizedAI

    ai = PersonalizedAI.__new__(PersonalizedAI)
    ai.profile = None

    result = ai.get_style_instruction("commit")
    assert result == ""


def test_get_style_instruction_skips_fallback_template(tmp_path, monkeypatch):
    """get_style_instruction() does not return a style directive for the fallback template."""
    monkeypatch.setenv("DATA_DIR", str(tmp_path))

    learning_dir = tmp_path / "learning"
    learning_dir.mkdir()
    fallback_text = (
        "# Developer Voice Profile\n\n"
        "Insufficient data for automated profiling. Run `devtrack voice add` to add examples manually."
    )
    (learning_dir / "profile.md").write_text(fallback_text, encoding="utf-8")

    from backend.personalized_ai import PersonalizedAI

    ai = PersonalizedAI.__new__(PersonalizedAI)
    ai.profile = None

    result = ai.get_style_instruction("commit")
    # Should fall through to "" since it's the fallback template
    assert result == ""
