"""
skill_detector.py — Detects emerging skill patterns from stored inferences.

Clusters inferences by normalized subject and promotes clusters that cross
EMERGENCE_THRESHOLD without any correction.  Called from webhook_server.py
after a batch of new inferences is stored.

Never raises — returns [] on any error so the calling endpoint is never broken
by skill-detection failures.
"""
import logging
import re
import sqlite3
from pathlib import Path
from typing import Any

from backend.config import get, get_int, get_path

logger = logging.getLogger(__name__)

# How many inferences sharing the same subject cluster must be observed
# before the cluster is promoted to a skill.  Must remain a named constant —
# do not use the literal integer 5 elsewhere in this module.
EMERGENCE_THRESHOLD = 5


class SkillDetector:
    """
    Detects recurring inference patterns and promotes them to skills by calling
    POST /dialectic/promote-skill on the Go daemon's internal HTTP API.
    Never raises — returns [] on any error.
    """

    def detect_and_promote(self, new_inferences: list[dict]) -> list[dict]:
        """
        Groups new_inferences by (context_type, normalized subject cluster).
        Normalisation: lowercase, strip punctuation, take first 4 words.

        For each cluster with len >= EMERGENCE_THRESHOLD where none of the
        inference IDs have a corrections row, calls _promote_skill().

        Returns list of promoted skill dicts (may be empty).
        Returns [] on any error without raising.
        """
        try:
            return self._detect(new_inferences)
        except Exception as e:
            logger.warning("SkillDetector.detect_and_promote error: %s", e)
            return []

    def _detect(self, new_inferences: list[dict]) -> list[dict]:
        # Group by (context_type, normalised cluster key).
        clusters: dict[tuple[str, str], list[dict]] = {}
        for inf in new_inferences:
            key = (
                inf.get("context_type", ""),
                self._normalize(inf.get("subject", "")),
            )
            clusters.setdefault(key, []).append(inf)

        # Only process clusters at or above threshold.
        candidates = {k: v for k, v in clusters.items() if len(v) >= EMERGENCE_THRESHOLD}
        if not candidates:
            return []

        # Open SQLite to check for corrections on candidate inference IDs.
        db_path = get_path("DATABASE_PATH")
        conn = sqlite3.connect(str(db_path))
        try:
            promoted: list[dict] = []
            for (context_type, cluster_key), infs in candidates.items():
                inference_ids = [i["id"] for i in infs if "id" in i]
                if not inference_ids:
                    continue
                placeholders = ",".join("?" * len(inference_ids))
                row = conn.execute(
                    f"SELECT COUNT(*) FROM corrections WHERE inference_id IN ({placeholders})",
                    inference_ids,
                ).fetchone()
                if row and row[0] > 0:
                    # At least one correction exists — skip promotion.
                    continue
                skill = self._promote_skill(cluster_key, context_type, len(infs))
                if skill:
                    promoted.append(skill)
            return promoted
        finally:
            conn.close()

    def _normalize(self, subject: str) -> str:
        """Lowercase, strip punctuation, take first 4 words."""
        s = subject.lower()
        s = re.sub(r"[^\w\s]", "", s)
        return " ".join(s.split()[:4])

    def _promote_skill(
        self,
        name: str,
        context_type: str,
        evidence_count: int,
    ) -> dict[str, Any] | None:
        """
        Calls POST /dialectic/promote-skill on the Go daemon's internal HTTP API.
        Returns the skill payload dict if successful, None otherwise.
        """
        try:
            import requests
        except ImportError:
            logger.warning("SkillDetector._promote_skill: 'requests' not installed")
            return None

        try:
            # The Go daemon's internal HTTP API runs on IPC_HOST:DEVTRACK_SERVER_HTTP_PORT
            # (default 127.0.0.1:35894) — NOT the Python server port 8089.
            ipc_host = get("IPC_HOST", "127.0.0.1")
            ipc_port = get_int("DEVTRACK_SERVER_HTTP_PORT", 35894)
            api_key = get("DEVTRACK_API_KEY", "")
            base_url = f"http://{ipc_host}:{ipc_port}"

            name_slug = name.replace(" ", "_")
            payload: dict[str, Any] = {
                "name": name_slug,
                "description": f"Developer pattern: {name} (context: {context_type})",
                "context_type": context_type,
                "evidence_count": evidence_count,
            }
            headers: dict[str, str] = {}
            if api_key:
                headers["X-DevTrack-API-Key"] = api_key

            resp = requests.post(
                f"{base_url}/dialectic/promote-skill",
                json=payload,
                headers=headers,
                timeout=5,
            )
            if resp.status_code in (200, 201):
                return payload
        except Exception as e:
            logger.warning("SkillDetector._promote_skill failed: %s", e)
        return None
