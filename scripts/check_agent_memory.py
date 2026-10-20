#!/usr/bin/env python3
"""Validate the single shared project-memory boundary."""

from __future__ import annotations

import os
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CANONICAL = ROOT / "agent-memory"
PROHIBITED_MEMORY_DIRS = (
    ROOT / ".claude" / "memory",
    ROOT / ".codex" / "memory",
    ROOT / ".agents" / "memory",
    ROOT / ".cursor" / "memory",
    ROOT / ".github" / "memory",
    ROOT / ".gemini" / "memory",
    ROOT / ".windsurf" / "memory",
)
ADAPTERS = (
    ROOT / "AGENTS.md",
    ROOT / "CLAUDE.md",
    ROOT / "devtrack_client" / "CLAUDE.md",
    ROOT / "devtrack_server" / "CLAUDE.md",
    ROOT / ".github" / "copilot-instructions.md",
)
REQUIRED_SHARED_FILES = (
    CANONICAL / "project-config.md",
    CANONICAL / "roles" / "project-vision.md",
    CANONICAL / "roles" / "devtrack-engineer.md",
    CANONICAL / "roles" / "git-agent.md",
    CANONICAL / "roles" / "docu-agent.md",
    CANONICAL / "roles" / "memory-compactor.md",
    CANONICAL / "roles" / "post-generator.md",
)
LEGACY_CLAUDE_SHARED_FILES = (
    ROOT / ".claude" / "pm-config.md",
    ROOT / ".claude" / "project_board.md",
    ROOT / ".claude" / "engineer_log.md",
    ROOT / ".claude" / "feature_tracker.md",
    ROOT / ".claude" / "commands" / "docu-agent.md",
    ROOT / ".claude" / "agents" / "_archive" / "project-vision.md",
    ROOT / ".claude" / "agents" / "_archive" / "devtrack-engineer.md",
    ROOT / ".claude" / "agents" / "_archive" / "git-agent.md",
    ROOT / ".claude" / "agents" / "_archive" / "memory-compactor.md",
    ROOT / ".claude" / "agents" / "_archive" / "post-generator.md",
)
SKIP_DIRS = {".git", ".pytest_cache", "__pycache__", ".venv", "node_modules"}
MARKDOWN_LINK = re.compile(r"\[[^\]]*\]\(([^)]+)\)")


def repository_files() -> list[Path]:
    files: list[Path] = []
    for current, directories, names in os.walk(ROOT):
        directories[:] = [name for name in directories if name not in SKIP_DIRS]
        base = Path(current)
        files.extend(base / name for name in names)
    return files


def main() -> int:
    errors: list[str] = []

    if not (CANONICAL / "INDEX.md").is_file():
        errors.append("agent-memory/INDEX.md is missing")

    for path in PROHIBITED_MEMORY_DIRS:
        if path.exists():
            errors.append(f"agent-specific memory directory is prohibited: {path.relative_to(ROOT)}")

    for path in REQUIRED_SHARED_FILES:
        if not path.is_file():
            errors.append(f"required shared agent file is missing: {path.relative_to(ROOT)}")

    for path in LEGACY_CLAUDE_SHARED_FILES:
        if path.exists():
            errors.append(f"legacy Claude-only shared file is prohibited: {path.relative_to(ROOT)}")

    for adapter in ADAPTERS:
        if not adapter.is_file():
            errors.append(f"agent discovery adapter is missing: {adapter.relative_to(ROOT)}")
            continue
        text = adapter.read_text(encoding="utf-8")
        if "agent-memory/INDEX.md" not in text:
            errors.append(f"adapter does not point to agent-memory/INDEX.md: {adapter.relative_to(ROOT)}")

    for memory_file in CANONICAL.rglob("*.md"):
        text = memory_file.read_text(encoding="utf-8")
        for raw_target in MARKDOWN_LINK.findall(text):
            target = raw_target.strip().split("#", 1)[0]
            if not target or "://" in target or target.startswith("mailto:"):
                continue
            resolved = (memory_file.parent / target).resolve()
            if not resolved.exists():
                errors.append(
                    f"broken shared-memory link in {memory_file.relative_to(ROOT)}: {raw_target}"
                )

    checker = Path(__file__).resolve()
    for path in repository_files():
        if path == checker:
            continue
        relative = path.relative_to(ROOT)
        if path.name.casefold() == "memory.md" and CANONICAL not in path.parents:
            errors.append(f"duplicate memory index outside agent-memory/: {relative}")
        if path.suffix.casefold() not in {".md", ".yaml", ".yml", ".json", ".toml"}:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        if ".claude/memory" in text or ".claude\\memory" in text:
            errors.append(f"stale Claude-specific memory reference: {relative}")

    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        return 1

    print("Shared agent memory boundary is valid.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
