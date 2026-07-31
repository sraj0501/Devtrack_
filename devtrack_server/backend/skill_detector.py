"""
skill_detector.py — Detects emerging skill patterns from stored inferences.

Clusters inferences by normalized subject and promotes clusters that cross
EMERGENCE_THRESHOLD without any correction.  Called from webhook_server.py
after a batch of new inferences is stored.

Never raises — returns [] on any error so the calling endpoint is never broken
by skill-detection failures.

Boundary rule
-------------
``inferences``/``corrections`` are Go-owned tables (created and written by the
Go daemon — see ``internal/db/migrations.go``).  This module never defines a
SQLAlchemy ``Table`` for them and never runs DDL against them — Python must
not create or own their schema (see the PostgreSQL Backend epic,
``Data/agent_logs/project_board.md``).

  SQLite mode     (POSTGRES_URL unset) — ``corrections`` lives in the same
    devtrack.db file the dual-dialect engine already points at, so the read
    goes through ``backend.db.engine.get_engine()`` instead of a bespoke
    ``sqlite3.connect()``.
  PostgreSQL mode (POSTGRES_URL set)   — Go never speaks Postgres (decided
    2026-07-13), so ``corrections`` does not exist there and there is no Go
    internal-HTTP endpoint exposing it yet (candidate for TASK-114). Skill
    detection is a no-op in this mode until that lands — fails closed
    (no promotions) rather than guessing, and still never raises.
"""
import logging
import re
from typing import Any

from sqlalchemy import text

from backend.config import get, get_int
from backend.db.engine import get_engine, is_postgres

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

        if is_postgres():
            # corrections is Go-owned and only ever exists in the client's local
            # SQLite file — there is no Go internal-HTTP endpoint exposing it yet
            # (see the boundary-rule note at the top of this module). Fail closed
            # rather than promote skills we can't verify against corrections.
            logger.debug(
                "SkillDetector: skipping promotion checks in PostgreSQL mode — "
                "corrections is a Go-owned table not yet exposed over HTTP (TASK-114)"
            )
            return []

        # Check for corrections on candidate inference IDs via the shared
        # SQLite-mode engine (same devtrack.db file the Go daemon writes).
        promoted: list[dict] = []
        with get_engine().connect() as conn:
            for (context_type, cluster_key), infs in candidates.items():
                inference_ids = [i["id"] for i in infs if "id" in i]
                if not inference_ids:
                    continue
                placeholders = ", ".join(f":id{i}" for i in range(len(inference_ids)))
                params = {f"id{i}": v for i, v in enumerate(inference_ids)}
                row = conn.execute(
                    text(
                        f"SELECT COUNT(*) FROM corrections WHERE inference_id IN ({placeholders})"
                    ),
                    params,
                ).fetchone()
                if row and row[0] > 0:
                    # At least one correction exists — skip promotion.
                    continue
                skill = self._promote_skill(cluster_key, context_type, len(infs))
                if skill:
                    promoted.append(skill)
        return promoted

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
