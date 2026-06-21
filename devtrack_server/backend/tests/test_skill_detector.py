"""Tests for SkillDetector (backend/skill_detector.py)."""
import pytest
from unittest.mock import MagicMock, patch

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


class TestSkillDetector:
    """Unit tests for SkillDetector.detect_and_promote."""

    def _make_db_mock(self, correction_count: int = 0):
        """Return a sqlite3.connect mock that reports correction_count corrections."""
        mock_cursor = MagicMock()
        mock_cursor.fetchone.return_value = (correction_count,)

        mock_conn = MagicMock()
        mock_conn.execute.return_value = mock_cursor
        # Support both context-manager and plain use (our code uses plain close()).
        mock_conn.__enter__ = MagicMock(return_value=mock_conn)
        mock_conn.__exit__ = MagicMock(return_value=False)

        return mock_conn

    def test_promotes_when_threshold_met_no_corrections(self):
        """EMERGENCE_THRESHOLD+1 inferences, same cluster, 0 corrections → promotes once."""
        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()
        mock_conn = self._make_db_mock(correction_count=0)

        with patch("backend.skill_detector.sqlite3.connect", return_value=mock_conn), \
             patch.object(
                 detector, "_promote_skill",
                 return_value={"name": "use_imperative_mood"}
             ) as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_promote.assert_called_once()
        assert len(result) == 1

    def test_no_promotion_when_corrections_exist(self):
        """EMERGENCE_THRESHOLD inferences but 1 correction → not promoted."""
        infs = make_inferences(EMERGENCE_THRESHOLD)
        detector = SkillDetector()
        mock_conn = self._make_db_mock(correction_count=1)

        with patch("backend.skill_detector.sqlite3.connect", return_value=mock_conn), \
             patch.object(detector, "_promote_skill") as mock_promote:
            result = detector.detect_and_promote(infs)

        mock_promote.assert_not_called()
        assert result == []

    def test_no_promotion_below_threshold(self):
        """EMERGENCE_THRESHOLD-1 inferences (below threshold) → not promoted, DB not queried."""
        infs = make_inferences(EMERGENCE_THRESHOLD - 1)
        detector = SkillDetector()

        with patch("backend.skill_detector.sqlite3.connect") as mock_connect, \
             patch.object(detector, "_promote_skill") as mock_promote:
            result = detector.detect_and_promote(infs)

        # DB should NOT be opened when no cluster reaches the threshold.
        mock_connect.assert_not_called()
        mock_promote.assert_not_called()
        assert result == []

    def test_returns_empty_list_on_db_error(self):
        """DB connection error → returns [] without raising."""
        infs = make_inferences(EMERGENCE_THRESHOLD + 1)
        detector = SkillDetector()

        with patch("backend.skill_detector.sqlite3.connect", side_effect=Exception("db error")):
            result = detector.detect_and_promote(infs)

        assert result == []

    def test_empty_input_returns_empty(self):
        """No inferences → empty list without touching DB."""
        detector = SkillDetector()

        with patch("backend.skill_detector.sqlite3.connect") as mock_connect:
            result = detector.detect_and_promote([])

        mock_connect.assert_not_called()
        assert result == []

    def test_two_clusters_promotes_both(self):
        """Two distinct subject clusters both at threshold with 0 corrections → two promotions."""
        infs_a = make_inferences(EMERGENCE_THRESHOLD, subject="use imperative mood", start_id=1)
        infs_b = make_inferences(EMERGENCE_THRESHOLD, subject="add ticket prefix commit", start_id=100)
        all_infs = infs_a + infs_b

        detector = SkillDetector()
        mock_conn = self._make_db_mock(correction_count=0)

        with patch("backend.skill_detector.sqlite3.connect", return_value=mock_conn), \
             patch.object(
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
        # Mock DB: 0 corrections for every inference in this cluster.
        mock_conn = self._make_db_mock(correction_count=0)

        with patch("backend.skill_detector.sqlite3.connect", return_value=mock_conn), \
             patch.object(
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
