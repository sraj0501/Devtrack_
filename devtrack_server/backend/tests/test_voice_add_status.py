"""
Tests for the POST /voice/add and GET /voice/status endpoints.

Acceptance criteria covered:
- POST /voice/add: valid text + context_type -> 200, returns {"id": "..."}
- POST /voice/add: invalid context_type -> 422
- POST /voice/add: missing text field -> 400
- GET /voice/status: empty corpus -> all counts 0, last_seed=null, profile_exists=false
- GET /voice/status: after adding one entry -> total_entries=1, correct by_context count
- GET /voice/status: ChromaDB unavailable -> returns zeros, no 500
"""
from __future__ import annotations

import sqlite3
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient


# ---------------------------------------------------------------------------
# Fixture: FastAPI test client
# ---------------------------------------------------------------------------

@pytest.fixture()
def client() -> TestClient:
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# POST /voice/add -- happy path
# ---------------------------------------------------------------------------

class TestVoiceAdd:
    """POST /voice/add endpoint tests."""

    def test_valid_text_and_context_returns_id(self, client: TestClient) -> None:
        """Valid request returns HTTP 200 with a non-empty id."""
        fake_vec = [0.1] * 768

        with (
            patch("backend.rag.embedder.embed", return_value=fake_vec),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
        ):
            resp = client.post(
                "/voice/add",
                json={"text": "Fixed the null check in auth flow", "context_type": "comment"},
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert "id" in body
        assert body["id"] != "", f"id should be non-empty, got: {body}"
        assert body["id"].startswith("manual-"), f"id should start with 'manual-', got: {body['id']}"

    def test_valid_commit_context(self, client: TestClient) -> None:
        """context_type=commit is accepted."""
        fake_vec = [0.2] * 768

        with (
            patch("backend.rag.embedder.embed", return_value=fake_vec),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
        ):
            resp = client.post(
                "/voice/add",
                json={"text": "add retry logic for HTTP client", "context_type": "commit"},
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert "id" in body
        assert body["id"].startswith("manual-")

    def test_all_valid_context_types_accepted(self, client: TestClient) -> None:
        """All five allowed context_type values are accepted."""
        valid_types = ["commit", "description", "comment", "report", "task"]
        fake_vec = [0.1] * 768

        for ct in valid_types:
            with (
                patch("backend.rag.embedder.embed", return_value=fake_vec),
                patch("backend.rag.vector_store.VectorStore._init", return_value=True),
                patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
            ):
                resp = client.post(
                    "/voice/add",
                    json={"text": f"example for {ct}", "context_type": ct},
                    headers={"X-DevTrack-API-Key": ""},
                )
            assert resp.status_code == 200, (
                f"context_type={ct!r} should be accepted, got {resp.status_code}: {resp.text}"
            )


# ---------------------------------------------------------------------------
# POST /voice/add -- validation failures
# ---------------------------------------------------------------------------

class TestVoiceAddValidation:
    """POST /voice/add rejects invalid inputs."""

    def test_invalid_context_type_returns_422(self, client: TestClient) -> None:
        """An unrecognized context_type returns HTTP 422."""
        resp = client.post(
            "/voice/add",
            json={"text": "some text", "context_type": "invalid_type"},
            headers={"X-DevTrack-API-Key": ""},
        )
        assert resp.status_code == 422, f"expected 422, got {resp.status_code}: {resp.text}"

    def test_missing_text_returns_400(self, client: TestClient) -> None:
        """Missing or empty text field returns HTTP 400."""
        resp = client.post(
            "/voice/add",
            json={"text": "", "context_type": "commit"},
            headers={"X-DevTrack-API-Key": ""},
        )
        assert resp.status_code == 400, f"expected 400, got {resp.status_code}: {resp.text}"

    def test_missing_text_key_returns_400(self, client: TestClient) -> None:
        """Body without a text key returns HTTP 400."""
        resp = client.post(
            "/voice/add",
            json={"context_type": "commit"},
            headers={"X-DevTrack-API-Key": ""},
        )
        assert resp.status_code == 400, f"expected 400, got {resp.status_code}: {resp.text}"

    def test_missing_context_type_returns_422(self, client: TestClient) -> None:
        """Missing context_type returns HTTP 422."""
        resp = client.post(
            "/voice/add",
            json={"text": "some text"},
            headers={"X-DevTrack-API-Key": ""},
        )
        assert resp.status_code == 422, f"expected 422, got {resp.status_code}: {resp.text}"


# ---------------------------------------------------------------------------
# POST /voice/add -- ChromaDB unavailable
# ---------------------------------------------------------------------------

class TestVoiceAddChromaUnavailable:
    """POST /voice/add degrades gracefully when ChromaDB is unavailable."""

    def test_embed_returns_none_gives_503(self, client: TestClient) -> None:
        """When embed() returns None, endpoint returns HTTP 503 without raising."""
        with patch("backend.rag.embedder.embed", return_value=None):
            resp = client.post(
                "/voice/add",
                json={"text": "example commit text", "context_type": "commit"},
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 503, f"expected 503, got {resp.status_code}: {resp.text}"
        body = resp.json()
        assert body.get("id") == ""
        assert "error" in body

    def test_upsert_false_gives_503(self, client: TestClient) -> None:
        """When upsert() returns False (ChromaDB write failed), endpoint returns 503."""
        fake_vec = [0.1] * 768
        with (
            patch("backend.rag.embedder.embed", return_value=fake_vec),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=False),
        ):
            resp = client.post(
                "/voice/add",
                json={"text": "example commit text", "context_type": "commit"},
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 503, f"expected 503, got {resp.status_code}: {resp.text}"

    def test_embed_raises_gives_503(self, client: TestClient) -> None:
        """When embed() raises an exception, endpoint returns 503 without a 500."""
        with patch("backend.rag.embedder.embed", side_effect=RuntimeError("chromadb down")):
            resp = client.post(
                "/voice/add",
                json={"text": "example commit text", "context_type": "commit"},
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 503, f"expected 503, got {resp.status_code}: {resp.text}"


# ---------------------------------------------------------------------------
# Helpers for GET /voice/status tests
#
# The endpoint uses local imports inside the function body, so we must patch
# at the source module ("backend.config") rather than at the webhook_server
# module level. We also patch VectorStore's internal _init + _collection.
# ---------------------------------------------------------------------------

def _make_mock_store(metadatas: list[dict]) -> MagicMock:
    """Return a mock VectorStore instance whose collection.get returns metadatas."""
    store = MagicMock()
    store._init.return_value = True
    store._collection = MagicMock()
    store._collection.get.return_value = {"metadatas": metadatas}
    return store


# ---------------------------------------------------------------------------
# GET /voice/status -- empty corpus
# ---------------------------------------------------------------------------

class TestVoiceStatusEmpty:
    """GET /voice/status returns zeros and nulls when no data exists."""

    def test_empty_corpus_all_zeros(self, client: TestClient) -> None:
        """Empty ChromaDB returns total_entries=0, all by_context/by_source=0."""
        mock_store = _make_mock_store([])

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["total_entries"] == 0
        for ct in ["commit", "description", "comment", "report", "task"]:
            assert body["by_context"].get(ct, 0) == 0
        assert body["last_seed"] is None
        assert body["profile_exists"] is False

    def test_status_structure_has_all_required_fields(self, client: TestClient) -> None:
        """Response always contains all documented fields."""
        mock_store = _make_mock_store([])
        mock_store._init.return_value = False  # pretend ChromaDB completely down

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        required = {
            "total_entries", "by_context", "by_source",
            "last_seed", "last_sync", "profile_exists", "profile_word_count",
        }
        missing = required - set(body.keys())
        assert not missing, f"response missing fields: {missing}"


# ---------------------------------------------------------------------------
# GET /voice/status -- after seeding
# ---------------------------------------------------------------------------

class TestVoiceStatusPopulated:
    """GET /voice/status reflects ChromaDB counts and SQLite timestamps."""

    def test_counts_from_metadata(self, client: TestClient) -> None:
        """Counts in by_context and by_source are derived from ChromaDB metadata."""
        fake_metadatas = [
            {"context_type": "commit", "source": "git_history"},
            {"context_type": "commit", "source": "git_history"},
            {"context_type": "comment", "source": "manual"},
        ]
        mock_store = _make_mock_store(fake_metadatas)

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["total_entries"] == 3
        assert body["by_context"]["commit"] == 2
        assert body["by_context"]["comment"] == 1
        assert body["by_source"]["git_history"] == 2
        assert body["by_source"]["manual"] == 1

    def test_last_seed_from_sqlite(self, client: TestClient, tmp_path: Path) -> None:
        """last_seed is populated from the voice_seeded_commits table."""
        db_path = tmp_path / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            "CREATE TABLE voice_seeded_commits "
            "(hash TEXT, repo_path TEXT, seeded_at DATETIME, PRIMARY KEY(hash, repo_path))"
        )
        conn.execute(
            "INSERT INTO voice_seeded_commits VALUES (?, ?, ?)",
            ("abc123", "/repo", "2026-06-18 10:00:00"),
        )
        conn.commit()
        conn.close()

        mock_store = _make_mock_store([])
        mock_store._init.return_value = False

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", return_value=db_path),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["last_seed"] is not None
        assert "2026-06-18" in body["last_seed"]

    def test_profile_exists_and_word_count(self, client: TestClient, tmp_path: Path) -> None:
        """profile_exists is True and profile_word_count is correct when file exists."""
        learning_dir = tmp_path / "learning"
        learning_dir.mkdir()
        profile_file = learning_dir / "profile.md"
        profile_text = "# Developer Voice Profile\n\nInformal, direct, imperative mood."
        profile_file.write_text(profile_text, encoding="utf-8")

        mock_store = _make_mock_store([])
        mock_store._init.return_value = False

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", return_value=tmp_path),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()
        assert body["profile_exists"] is True
        expected_words = len(profile_text.split())
        assert body["profile_word_count"] == expected_words


# ---------------------------------------------------------------------------
# GET /voice/status -- ChromaDB unavailable (no 500)
# ---------------------------------------------------------------------------

class TestVoiceStatusChromaUnavailable:
    """GET /voice/status returns zeros when ChromaDB is completely unavailable."""

    def test_chroma_init_fails_no_500(self, client: TestClient) -> None:
        """When ChromaDB init fails, status endpoint returns 200 with zeros -- never 500."""
        # Make VectorStore constructor raise to simulate import failure.
        with (
            patch(
                "backend.rag.vector_store.VectorStore",
                side_effect=Exception("chromadb not installed"),
            ),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, f"expected 200, got {resp.status_code}: {resp.text}"
        body = resp.json()
        assert body["total_entries"] == 0
        assert body["profile_exists"] is False

    def test_chroma_import_error_no_500(self, client: TestClient) -> None:
        """Even when VectorStore import itself fails, status returns 200 with zeros."""
        with (
            patch(
                "backend.rag.vector_store.VectorStore",
                side_effect=ImportError("chromadb not available"),
            ),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, f"expected 200, got {resp.status_code}: {resp.text}"
        body = resp.json()
        assert body["total_entries"] == 0


# ---------------------------------------------------------------------------
# GET /voice/status -- Phase 6 dialectic fields (TASK-091)
# ---------------------------------------------------------------------------

class TestVoiceStatusDialecticFields:
    """GET /voice/status includes inferences, skills, thresholds from DialecticStatus."""

    def test_voice_status_includes_dialectic_fields_with_mocked_db(
        self, client: TestClient
    ) -> None:
        """Response includes inferences, skills, thresholds when DialecticStatus is mocked."""
        mock_inf = {
            "total": 2,
            "top_by_confidence": [
                {
                    "id": 1,
                    "subject": "tone",
                    "inference": "imperative",
                    "confidence": 0.91,
                    "context_type": "commit",
                }
            ],
            "correction_count": 0,
        }
        mock_skills = {"total": 1, "names": ["imperative_commit_tone"]}
        mock_thresholds: dict = {}

        mock_store = _make_mock_store([])
        mock_store._init.return_value = False

        with (
            patch("backend.rag.vector_store.VectorStore", return_value=mock_store),
            patch("backend.config.database_path", side_effect=Exception("no db")),
            patch("backend.config.get_path", side_effect=Exception("no data_dir")),
            patch(
                "backend.dialectic_status.DialecticStatus.get_inference_summary",
                return_value=mock_inf,
            ),
            patch(
                "backend.dialectic_status.DialecticStatus.get_skill_summary",
                return_value=mock_skills,
            ),
            patch(
                "backend.dialectic_status.DialecticStatus.get_threshold_summary",
                return_value=mock_thresholds,
            ),
        ):
            resp = client.get(
                "/voice/status",
                headers={"X-DevTrack-API-Key": ""},
            )

        assert resp.status_code == 200, resp.text
        body = resp.json()

        # inferences block
        assert "inferences" in body, "response should contain 'inferences' key"
        assert body["inferences"]["total"] == 2
        assert len(body["inferences"]["top_by_confidence"]) == 1
        entry = body["inferences"]["top_by_confidence"][0]
        assert entry["subject"] == "tone"
        assert entry["confidence"] == 0.91
        assert entry["context_type"] == "commit"

        # skills block
        assert "skills" in body, "response should contain 'skills' key"
        assert body["skills"]["total"] == 1
        assert "imperative_commit_tone" in body["skills"]["names"]

        # thresholds block present (even if empty)
        assert "thresholds" in body, "response should contain 'thresholds' key"


# ---------------------------------------------------------------------------
# DialecticStatus unit tests (TASK-091)
# ---------------------------------------------------------------------------

class TestDialecticStatusUnit:
    """Unit tests for DialecticStatus helper methods."""

    def test_get_inference_summary_nonexistent_db_returns_safe_default(
        self, tmp_path: Path
    ) -> None:
        """get_inference_summary() with a nonexistent DB path returns zeros without raising."""
        from backend.dialectic_status import DialecticStatus

        nonexistent = tmp_path / "does_not_exist.db"

        # Patch the database_path name in the dialectic_status module's own namespace.
        with patch("backend.dialectic_status.database_path", return_value=nonexistent):
            ds = DialecticStatus()
            result = ds.get_inference_summary()

        assert result == {
            "total": 0,
            "top_by_confidence": [],
            "correction_count": 0,
        }, f"expected safe default, got: {result}"

    def test_get_inference_summary_with_real_db(self, tmp_path: Path) -> None:
        """get_inference_summary() returns correct counts from a real SQLite DB."""
        import sqlite3 as _sqlite3
        from backend.dialectic_status import DialecticStatus

        db_path = tmp_path / "devtrack.db"
        conn = _sqlite3.connect(str(db_path))
        conn.execute(
            """
            CREATE TABLE inferences (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                context_type TEXT NOT NULL,
                subject TEXT NOT NULL,
                inference TEXT NOT NULL,
                evidence TEXT NOT NULL DEFAULT '[]',
                confidence REAL NOT NULL DEFAULT 0.5,
                source TEXT NOT NULL DEFAULT 'hermes3',
                created_at DATETIME NOT NULL DEFAULT (datetime('now')),
                updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
            )
            """
        )
        conn.execute(
            """
            CREATE TABLE corrections (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                inference_id INTEGER NOT NULL,
                correction TEXT NOT NULL,
                flagged_from TEXT NOT NULL DEFAULT 'tui',
                weight REAL NOT NULL DEFAULT 2.0,
                created_at DATETIME NOT NULL DEFAULT (datetime('now'))
            )
            """
        )
        # Insert 2 inferences.
        conn.execute(
            "INSERT INTO inferences (context_type, subject, inference, evidence, confidence)"
            " VALUES (?, ?, ?, ?, ?)",
            ("commit", "tone", "Uses imperative mood.", "[]", 0.91),
        )
        conn.execute(
            "INSERT INTO inferences (context_type, subject, inference, evidence, confidence)"
            " VALUES (?, ?, ?, ?, ?)",
            ("comment", "prefix", "Brackets ticket ID.", "[]", 0.87),
        )
        # Insert 1 correction.
        conn.execute(
            "INSERT INTO corrections (inference_id, correction) VALUES (?, ?)",
            (1, "Actually uses past tense sometimes."),
        )
        conn.commit()
        conn.close()

        # Patch database_path in the dialectic_status module namespace so
        # _resolve_db_path() returns our real test DB.
        with patch("backend.dialectic_status.database_path", return_value=db_path):
            ds = DialecticStatus()
            result = ds.get_inference_summary()

        assert result["total"] == 2, f"expected total=2, got {result['total']}"
        assert result["correction_count"] == 1, (
            f"expected correction_count=1, got {result['correction_count']}"
        )
        assert len(result["top_by_confidence"]) == 2
        # First entry should be highest confidence (0.91).
        assert result["top_by_confidence"][0]["confidence"] == 0.91
