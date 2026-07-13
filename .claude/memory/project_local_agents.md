---
name: Agent system
description: 5 agents defined in .claude/agents/_archive/ (still active despite the name); pm-config.md drives project values
type: project
---

**Agents live at `.claude/agents/_archive/`** (5): project-vision, devtrack-engineer, git-agent, memory-compactor, post-generator. Despite the `_archive` directory name these are the **live** definitions — Claude Code recurses into it, and there is **no `~/.claude/agents/`** on this machine. [Verified 2026-07-13; an older memory claiming "6 global agents at `~/.claude/agents/`" was wrong.]

**Project values:** `.claude/pm-config.md` (repo root).

**Rules:** project-vision always first (creates the board task) → devtrack-engineer (only when dispatched) → git-agent for push/PR (never commits). memory-compactor: weekly. post-generator: weekly engineer log → posts.

**Runtime files** (gitignored): `Data/agent_logs/{project_board,engineer_log,feature_tracker}.md`.
