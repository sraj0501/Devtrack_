"""Tests for SkillDetector (backend/skill_detector.py).

skill_detector.py reads the Go-owned ``corrections`` table through
``backend.db.engine.get_engine()`` (SQLite mode) rather than a bespoke
``sqlite3.connect()``. These tests exercise that path against a real,
temp-directory SQLite DB — created and migrated the way pytest fixtures for
other engine.py-backed modules do (see test_project_manager.py) — plus the
PostgreSQL-mode no-op path via a monkeypatched ``is_postgres()``.
"""
from unittest.mock import patch

import pytest
from sqlalchemy import text

from backend.db.engine import get_engine, reset_engine
from backend.skill_detector import SkillDetector, EMERGENCE_THRESHOLD


def make_inferences(
    n: int,
    context_type: str = "commit",
    subject: str = "use imperative mood in commits",
    start_id: int = 1,
) -> list[dict]:
    """Build a list of n inference dicts sharing the same subject cluster."""
    return [
        {"id": start_id + i, "context_type": context_type, "subject": subject}
        for i in range(n)
    ]


@pytest.fixture(autouse=True)
def isolated_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the engine
    singleton so each test gets its own SQLite file, then create the
    Go-owned ``corrections`` table by hand (Python never owns its schema —
    see the boundary-rule docstring in skill_detector.py).
    """
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    with get_engine().connect() as conn:
        conn.execute(
            text(
                "CREATE TABLE IF NOT EXISTS corrections ("
                "id INTEGER PRIMARY KEY AUTOINCREMENT, "
                "inference_id INTEGER NOT NULL, "
                "correction TEXT, "
                "flagged_from TEXT, "
                "weight REAL"
                ")"
            )
        )
        conn.commit()
    yield
    reset_engine()


def add_correction(inference_id: int) -> None:
    with get_engine().connect() as conn:
        conn.execute(
            text("INSERT INTO corrections (inference_id, correction) VALUES (:iid, :c)"),
            {"iid": inference_id, "c": "developer edited this"},
        )
        conn.commit()


class TestSkillDetector:
    """Unit tests for SkillDetector.detect_and_promote."""

    def test_promotes_when_threshold_met_no_corrections(self):
        """EMERGENCE_THRESHOLD+1 inferences, same cluster, 0 corrections → promotes once."""
        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()

        with patch.object(
            detector, "_promote_skill",
            return_value={"name": "use_imperative_mood"}
        ) as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_promote.assert_called_once()
        assert len(result) == 1

    def test_no_promotion_when_corrections_exist(self):
        """EMERGENCE_THRESHOLD inferences but 1 correction → not promoted."""
        infs = make_inferences(EMERGENCE_THRESHOLD)
        add_correction(infs[0]["id"])
        detector = SkillDetector()

        with patch.object(detector, "_promote_skill") as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_promote.assert_not_called()
        assert result == []

    def test_no_promotion_below_threshold(self):
        """EMERGENCE_THRESHOLD-1 inferences (below threshold) → not promoted, DB not queried."""
        infs = make_inferences(EMERGENCE_THRESHOLD - 1)
        detector = SkillDetector()

        with patch.object(detector, "_promote_skill") as mock_promote, \
             patch("backend.skill_detector.get_engine") as mock_get_engine:
            result = detector.detect_and_promote(infs)

        # DB should NOT be opened when no cluster reaches the threshold.
        mock_get_engine.assert_not_called()
        mock_promote.assert_not_called()
        assert result == []

    def test_returns_empty_list_on_db_error(self):
        """DB connection error → returns [] without raising."""
        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()

        with patch("backend.skill_detector.get_engine", side_effect=Exception("db error")):
            result = detector.detect_and_promote(infs)

        assert result == []

    def test_missing_corrections_table_returns_empty(self):
        """corrections table absent (e.g. Go daemon never migrated) → [] without raising."""
        with get_engine().connect() as conn:
            conn.execute(text("DROP TABLE corrections"))
            conn.commit()

        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()

        with patch.object(detector, "_promote_skill") as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_promote.assert_not_called()
        assert result == []

    def test_empty_input_returns_empty(self):
        """No inferences → empty list without touching DB."""
        detector = SkillDetector()

        with patch("backend.skill_detector.get_engine") as mock_get_engine:
            result = detector.detect_and_promote([])

        mock_get_engine.assert_not_called()
        assert result == []

    def test_two_clusters_promotes_both(self):
        """Two distinct subject clusters both at threshold with 0 corrections → two promotions."""
        infs_a = make_inferences(EMERGENCE_THRESHOLD, subject="use imperative mood", start_id=1)
        infs_b = make_inferences(EMERGENCE_THRESHOLD, subject="add ticket prefix commit", start_id=100)
        all_infs = infs_a + infs_b

        detector = SkillDetector()

        with patch.object(
            detector, "_promote_skill",
            side_effect=lambda name, ctx, ev: {"name": name}
        ) as mock_promote:
            result = detector.detect_and_promote(all_infs)

        assert mock_promote.call_count == 2
        assert len(result) == 2

    def test_normalize_strips_punctuation_and_truncates(self):
        """_normalize produces consistent cluster keys."""
        detector = SkillDetector()
        assert detector._normalize("Use imperative mood!") == "use imperative mood"
        assert detector._normalize("  CAPS   and  spaces  ") == "caps and spaces"
        # Only first 4 words kept.
        assert detector._normalize("one two three four five six") == "one two three four"

    def test_commit_tone_skill_emergence_simulation(self):
        """Phase 6 exit criterion verification — skill emergence simulation.

        5 inferences with identical subject 'commit_tone' and context_type='commit',
        no corrections → detect_and_promote returns a promoted skill entry and
        the _promote_skill endpoint is called exactly once.

        This simulates a 30-day sequence where 'commit_tone' is the canonical
        subject cluster that crosses EMERGENCE_THRESHOLD (5) without correction.
        """
        # Build exactly EMERGENCE_THRESHOLD inferences sharing the same cluster.
        inferences = [
            {"id": i + 1, "context_type": "commit", "subject": "commit_tone"}
            for i in range(EMERGENCE_THRESHOLD)
        ]

        detector = SkillDetector()

        with patch.object(
            detector,
            "_promote_skill",
            return_value={"name": "commit_tone"},
        ) as mock_promote:
            result = detector.detect_and_promote(inferences)

        # The promote endpoint must have been called once (one cluster).
        mock_promote.assert_called_once()
        # The returned list must contain one entry — the "commit_tone" skill.
        assert len(result) == 1, f"Expected 1 promoted skill, got {len(result)}: {result}"
        assert result[0]["name"] == "commit_tone", (
            f"Expected skill name 'commit_tone', got {result[0]['name']!r}"
        )


class TestSkillDetectorPostgresMode:
    """PostgreSQL-mode boundary-rule behaviour: corrections is Go-owned and not
    yet exposed over HTTP (TASK-114), so detection must no-op, not crash."""

    def test_postgres_mode_skips_promotion_without_touching_engine(self):
        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()

        with patch("backend.skill_detector.is_postgres", return_value=True), \
             patch("backend.skill_detector.get_engine") as mock_get_engine, \
             patch.object(detector, "_promote_skill") as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_get_engine.assert_not_called()
        mock_promote.assert_not_called()
        assert result == []
