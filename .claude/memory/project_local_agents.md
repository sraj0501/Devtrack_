---
name: Agent system
description: Local project agents (archived); pm-config.md drives project-specific values
type: project
---

**Agents** at `.claude/agents/` (all archived in `_archive/`): devtrack-engineer, git-agent, memory-compactor, post-generator, project-vision.
**pm-config:** `.claude/pm-config.md` drives project-specific values.

**Rules when active:** project-vision first (creates board task) → engineer (only when dispatched) → git-agent for push/PR (never commits).

**Runtime files** (gitignored): `.claude/engineer_log.md`, `.claude/project_board.md`.

**Bootstrap:** `/init` — generates CLAUDE.md + pm-config. Fill `vision.rules` + `posts.author` manually.
