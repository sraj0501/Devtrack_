"""
Vacation mode auto-responder.

When vacation mode is active, this module replaces the interactive TUI prompt:
it generates a work update from recent commits using the LLM, scores confidence,
and (optionally) auto-submits it to the configured PM platform.
"""

from __future__ import annotations

import logging
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Optional

from backend.config import get, get_float, get_bool, get_path, ConfigError

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Vacation state helpers
# ---------------------------------------------------------------------------

@dataclass
class VacationState:
    enabled: bool
    enabled_at: str
    until: str          # empty = indefinite
    confidence_threshold: float
    auto_submit: bool


def get_vacation_state() -> Optional[VacationState]:
    """Read vacation mode state from devtrack.db. Returns None on any error."""
    try:
        db_path = get_path("DATABASE_DIR") / get("DATABASE_FILE_NAME")
        conn = sqlite3.connect(str(db_path))
        row = conn.execute(
            "SELECT enabled, enabled_at, until, confidence_threshold, auto_submit "
            "FROM vacation_mode WHERE id = 1"
        ).fetchone()
        conn.close()
        if row is None:
            return None
        return VacationState(
            enabled=bool(row[0]),
            enabled_at=row[1] or "",
            until=row[2] or "",
            confidence_threshold=float(row[3]) if row[3] else 0.7,
            auto_submit=bool(row[4]),
        )
    except Exception as e:
        logger.debug("Could not read vacation state: %s", e)
        return None


def is_vacation_active() -> bool:
    """Return True if vacation mode is currently active (and not expired)."""
    state = get_vacation_state()
    if state is None or not state.enabled:
        return False
    if state.until:
        try:
            until_dt = datetime.strptime(state.until, "%Y-%m-%d").replace(tzinfo=timezone.utc)
            # Treat the entire until-day as valid
            if datetime.now(timezone.utc).date() > until_dt.date():
                return False
        except ValueError:
            pass
    return True


# ---------------------------------------------------------------------------
# Auto-responder
# ---------------------------------------------------------------------------

class VacationAutoResponder:
    """
    Generates and optionally submits a work update autonomously when
    vacation mode is active.
    """

    def __init__(self) -> None:
        self.state = get_vacation_state()

    async def handle(
        self,
        trigger_data: dict,
        project_manager=None,
    ) -> dict:
        """
        Main entry point called from webhook_server.process_timer() when
        vacation mode is active.

        Returns a dict with:
          - generated_update: str
          - confidence: float (0-1)
          - submitted: bool
          - skipped_reason: str | None
        """
        if not self.state or not self.state.enabled:
            return {"skipped_reason": "vacation mode not active"}

        commits = trigger_data.get("recent_commits", [])
        workspace = trigger_data.get("workspace_name", "")
        branch = trigger_data.get("branch", "")

        # Build context for the LLM
        context = self._build_context(commits, workspace, branch)

        # Generate update + confidence score
        update, confidence = await self._generate_update(context)
        if not update:
            return {"skipped_reason": "LLM generation failed", "confidence": 0.0, "submitted": False}

        threshold = self.state.confidence_threshold
        logger.info(
            "Vacation auto-responder: confidence=%.2f threshold=%.2f workspace=%s",
            confidence, threshold, workspace,
        )

        submitted = False
        if confidence >= threshold and self.state.auto_submit and project_manager:
            try:
                await self._submit_update(update, trigger_data, project_manager)
                submitted = True
                logger.info("Vacation auto-responder: update submitted to PM")
            except Exception as e:
                logger.warning("Vacation auto-responder: PM submit failed: %s", e)
        elif confidence < threshold:
            logger.info(
                "Vacation auto-responder: confidence %.0f%% below threshold %.0f%% — skipping submit",
                confidence * 100, threshold * 100,
            )

        return {
            "generated_update": update,
            "confidence": confidence,
            "submitted": submitted,
            "skipped_reason": None if confidence >= threshold else "confidence below threshold",
        }

    # ------------------------------------------------------------------

    def _build_context(self, commits: list, workspace: str, branch: str) -> str:
        lines = []
        if workspace:
            lines.append(f"Workspace: {workspace}")
        if branch:
            lines.append(f"Branch: {branch}")
        if commits:
            lines.append("\nRecent commits:")
            for c in commits[:10]:
                msg = c.get("message", "") if isinstance(c, dict) else str(c)
                lines.append(f"  - {msg}")
        else:
            lines.append("\nNo recent commits.")
        return "\n".join(lines)

    async def _generate_update(self, context: str) -> tuple[str, float]:
        """Call LLM to generate a work update and estimate confidence (0-1)."""
        try:
            from backend.llm.provider_factory import get_llm_provider
            provider = get_llm_provider()

            prompt = f"""You are an AI assistant generating a brief work status update on behalf of a developer who is on vacation.
Based on the git activity below, write a concise 1-3 sentence work update suitable for a project management system.
Then, on a new line, output CONFIDENCE: followed by a number from 0.0 to 1.0 indicating how confident you are
that this update accurately reflects the work done (low confidence if there are no commits or very little context).

{context}

Work update:"""

            response = await provider.generate(prompt, max_tokens=200, temperature=0.3)
            return self._parse_response(response)
        except Exception as e:
            logger.warning("Vacation auto-responder LLM error: %s", e)
            return "", 0.0

    def _parse_response(self, response: str) -> tuple[str, float]:
        """Extract update text and confidence score from LLM response."""
        confidence = 0.5  # default if not found
        lines = response.strip().splitlines()
        update_lines = []
        for line in lines:
            if line.upper().startswith("CONFIDENCE:"):
                try:
                    confidence = float(line.split(":", 1)[1].strip())
                    confidence = max(0.0, min(1.0, confidence))
                except ValueError:
                    pass
            else:
                update_lines.append(line)
        update = "\n".join(update_lines).strip()
        # Penalise if update is very short (low info)
        if len(update) < 20:
            confidence = min(confidence, 0.3)
        return update, confidence

    async def _submit_update(self, update: str, trigger_data: dict, project_manager) -> None:
        """Submit the generated update to the PM system."""
        workspace_name = trigger_data.get("workspace_name", "")
        ticket_id = trigger_data.get("ticket_id", "")
        await project_manager.update_task(
            description=f"[Auto — vacation mode] {update}",
            ticket_id=ticket_id,
            workspace_name=workspace_name,
        )
