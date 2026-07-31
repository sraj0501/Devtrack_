"""
Tests for backend.voice_seeder.VoiceSeeder.

Covers:
1. seed_from_git returns correct count; merge commits are skipped.
2. Idempotent: second call with same hashes returns 0.
3. Git unavailable (subprocess raises FileNotFoundError): returns 0, no exception.
4. ChromaDB unavailable (embed returns None): returns 0, no exception.

voice_seeded_commits is a Python-owned table (backend.db.voice_seed_store,
TASK-112 port) on the shared SQLAlchemy engine -- tests use the DATABASE_DIR
+ reset_engine() isolated-engine fixture (same pattern as
test_skill_detector.py/test_work_tracker.py) instead of patching a
_db_path()-style helper, since get_engine() is a process-wide singleton that
resolves its own path and would ignore an explicitly-passed one.
"""
from __future__ import annotations

import subprocess
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from backend.db.engine import reset_engine


# ---------------------------------------------------------------------------
# Helpers / fixtures
# ---------------------------------------------------------------------------

FAKE_GIT_LOG = (
    "abc1234|Fix null pointer in auth module\n"
    "def5678|Merge branch feature-x into main\n"
    "ghi9012|Merge pull request #99 from user/feature-y\n"
    "jkl3456|Add retry logic for API calls\n"
    "mno7890|Refactor session handling\n"
)


def _make_completed_process(stdout: str, returncode: int = 0) -> subprocess.CompletedProcess:
    cp = subprocess.CompletedProcess(args=[], returncode=returncode)
    cp.stdout = stdout
    cp.stderr = ""
    return cp


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    import backend.db.voice_seed_store as vss

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    vss._schema_done = False
    yield tmp_path
    reset_engine()
    vss._schema_done = False


# ---------------------------------------------------------------------------
# 1. Correct count; merge commits skipped
# ---------------------------------------------------------------------------

class TestSeedFromGit:
    """seed_from_git counts non-merge commits correctly."""

    def test_correct_count_and_merge_skipped(self, isolated_engine: Path, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        fake_vec = [0.1] * 768

        with (
            patch("subprocess.run", return_value=_make_completed_process(FAKE_GIT_LOG)),
            patch("backend.rag.embedder.embed", return_value=fake_vec),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
        ):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        # 5 lines in fake log, 2 are merges, so 3 real commits
        assert count == 3, f"expected 3 but got {count}"

    def test_merge_commits_are_skipped(self, isolated_engine: Path, tmp_path: Path) -> None:
        """Merge branch / Merge pull request lines must not be embedded."""
        only_merges = (
            "aaa1111|Merge branch feature-a into main\n"
            "bbb2222|Merge pull request #1 from user/branch\n"
        )

        with (
            patch("subprocess.run", return_value=_make_completed_process(only_merges)),
            patch("backend.rag.embedder.embed", return_value=[0.1] * 768),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
        ):
            from backend.voice_seeder import VoiceSeeder
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0


# ---------------------------------------------------------------------------
# 2. Idempotent: second call returns 0
# ---------------------------------------------------------------------------

class TestIdempotency:
    """Second call with same hashes must return 0 (already seeded)."""

    def test_second_call_returns_zero(self, isolated_engine: Path, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        fake_vec = [0.1] * 768
        log_output = "abc1234|Add retry logic\nmno7890|Refactor session\n"

        common_patches = dict(
            subprocess_run=patch("subprocess.run", return_value=_make_completed_process(log_output)),
            embed=patch("backend.rag.embedder.embed", return_value=fake_vec),
            upsert=patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
            init=patch("backend.rag.vector_store.VectorStore._init", return_value=True),
        )

        seeder = VoiceSeeder()

        # First call
        with (
            common_patches["subprocess_run"],
            common_patches["embed"],
            common_patches["upsert"],
            common_patches["init"],
        ):
            first = seeder.seed_from_git(str(tmp_path), since_months=6)

        # Second call — same git output, same DB
        with (
            patch("subprocess.run", return_value=_make_completed_process(log_output)),
            patch("backend.rag.embedder.embed", return_value=fake_vec),
            patch("backend.rag.vector_store.VectorStore.upsert", return_value=True),
            patch("backend.rag.vector_store.VectorStore._init", return_value=True),
        ):
            second = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert first == 2, f"first call should embed 2, got {first}"
        assert second == 0, f"second call should be 0 (idempotent), got {second}"


# ---------------------------------------------------------------------------
# 3. Git unavailable
# ---------------------------------------------------------------------------

class TestGitUnavailable:
    """When git is not found, seed_from_git must return 0 without raising."""

    def test_file_not_found_returns_zero(self, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        with patch("subprocess.run", side_effect=FileNotFoundError("git not found")):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0

    def test_timeout_returns_zero(self, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired(cmd="git", timeout=30)):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0

    def test_nonzero_exit_returns_zero(self, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        bad_result = _make_completed_process("", returncode=128)
        with patch("subprocess.run", return_value=bad_result):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0


# ---------------------------------------------------------------------------
# 4. ChromaDB unavailable (embed returns None)
# ---------------------------------------------------------------------------

class TestChromaDBUnavailable:
    """When embed() returns None (Ollama/ChromaDB down), return 0 without raising."""

    def test_embed_none_returns_zero(self, isolated_engine: Path, tmp_path: Path) -> None:
        from backend.voice_seeder import VoiceSeeder

        log_output = "abc1234|Add retry logic\n"

        with (
            patch("subprocess.run", return_value=_make_completed_process(log_output)),
            patch("backend.rag.embedder.embed", return_value=None),
        ):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0

    def test_chroma_import_error_returns_zero(self, isolated_engine: Path, tmp_path: Path) -> None:
        """When backend.rag.embedder cannot be imported, return 0."""
        from backend.voice_seeder import VoiceSeeder

        log_output = "abc1234|Add retry logic\n"

        def _bad_embed(*args, **kwargs):
            raise ImportError("chromadb not installed")

        with (
            patch("subprocess.run", return_value=_make_completed_process(log_output)),
            patch("backend.voice_seeder.VoiceSeeder._embed_commit", side_effect=_bad_embed),
        ):
            seeder = VoiceSeeder()
            count = seeder.seed_from_git(str(tmp_path), since_months=6)

        assert count == 0
