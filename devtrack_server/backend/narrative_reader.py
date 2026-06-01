"""
narrative_reader.py — Parse narrative.log for admin UI and CLI consumption.

narrative.log is one JSON object per line (runtime-narrative JsonRenderer output).
Events for one request are linked by story_id:
  StoryStarted → StageStarted / StageCompleted (×N) → [FailureOccurred] → StoryCompleted
"""

from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional


@dataclass
class StageRecord:
    name: str
    duration_ms: Optional[float]  # None for failed stages (no completion event)
    failed: bool = False


@dataclass
class StoryRecord:
    story_id: str
    name: str
    started_at: str       # ISO timestamp string
    completed_at: Optional[str]
    success: Optional[bool]
    stages: list[StageRecord] = field(default_factory=list)
    failure: Optional[dict] = None   # full FailureOccurred event dict
    total_stages: int = 0
    completed_stages: int = 0

    @property
    def duration_ms(self) -> Optional[float]:
        if not self.started_at or not self.completed_at:
            return None
        try:
            t0 = datetime.fromisoformat(self.started_at)
            t1 = datetime.fromisoformat(self.completed_at)
            return (t1 - t0).total_seconds() * 1000
        except Exception:
            return None

    def to_dict(self) -> dict:
        return {
            "story_id":        self.story_id,
            "name":            self.name,
            "started_at":      self.started_at,
            "completed_at":    self.completed_at,
            "success":         self.success,
            "duration_ms":     self.duration_ms,
            "total_stages":    self.total_stages,
            "completed_stages": self.completed_stages,
            "stages": [
                {"name": s.name, "duration_ms": s.duration_ms, "failed": s.failed}
                for s in self.stages
            ],
            "failure": self.failure,
        }


def _log_path() -> str:
    log_dir = os.environ.get("LOG_DIR", "Data/logs")
    return os.environ.get(
        "NARRATIVE_LOG_PATH",
        os.path.join(log_dir, "narrative.log"),
    )


def _parse_log(path: str) -> tuple[dict[str, dict], list[str]]:
    """Read narrative.log and return (stories_map, completion_order)."""
    stories: dict[str, dict] = {}
    order: list[str] = []

    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    ev = json.loads(line)
                except json.JSONDecodeError:
                    continue

                sid = ev.get("story_id")
                event = ev.get("event", "")
                if not sid:
                    continue

                if event == "StoryStarted":
                    stories[sid] = {
                        "story_id": sid,
                        "name": ev.get("story_name", ""),
                        "started_at": ev.get("timestamp", ""),
                        "completed_at": None,
                        "success": None,
                        "stages": [],
                        "failure": None,
                        "total_stages": 0,
                        "completed_stages": 0,
                    }

                elif event == "StageCompleted" and sid in stories:
                    stories[sid]["stages"].append({
                        "name": ev.get("stage_name", ""),
                        "duration_ms": (ev.get("duration_seconds") or 0) * 1000,
                        "failed": False,
                    })

                elif event == "FailureOccurred" and sid in stories:
                    stories[sid]["stages"].append({
                        "name": ev.get("stage_name", ""),
                        "duration_ms": None,
                        "failed": True,
                    })
                    stories[sid]["failure"] = ev

                elif event == "StoryCompleted" and sid in stories:
                    stories[sid]["completed_at"] = ev.get("timestamp", "")
                    stories[sid]["success"] = ev.get("success", True)
                    p = ev.get("progress", {})
                    stories[sid]["total_stages"] = p.get("total_stages", 0)
                    stories[sid]["completed_stages"] = p.get("completed_stages", 0)
                    order.append(sid)

    except OSError:
        pass

    return stories, order


def get_recent_stories(n: int = 20) -> list[StoryRecord]:
    """Return the last n completed stories, newest first."""
    path = _log_path()
    if not os.path.exists(path):
        return []

    stories, order = _parse_log(path)

    result: list[StoryRecord] = []
    for sid in reversed(order):
        s = stories.get(sid)
        if not s:
            continue
        result.append(StoryRecord(
            story_id=s["story_id"],
            name=s["name"],
            started_at=s["started_at"],
            completed_at=s["completed_at"],
            success=s["success"],
            stages=[StageRecord(**st) for st in s["stages"]],
            failure=s["failure"],
            total_stages=s["total_stages"],
            completed_stages=s["completed_stages"],
        ))
        if len(result) >= n:
            break

    return result


def get_last_failure() -> Optional[dict]:
    """Return the most recent FailureOccurred event dict, or None."""
    path = _log_path()
    if not os.path.exists(path):
        return None

    last: Optional[dict] = None
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    ev = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if ev.get("event") == "FailureOccurred":
                    last = ev
    except OSError:
        pass

    return last
