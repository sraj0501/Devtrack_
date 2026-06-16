---
name: Agent system
description: 6 global agents at ~/.claude/agents/; project-level copies archived; pm-config.md drives project-specific values
type: project
---

**Agents live globally** at `~/.claude/agents/` (6): project-vision, engineer, git-agent, memory-compactor, post-generator, production-engineer. Project-level copies are archived/unused at `.claude/agents/_archive/`.
**pm-config:** `.claude/pm-config.md` (project root)

**Rules:** project-vision always first (creates board task) → engineer (only when dispatched) → git-agent for push/PR (never commits). memory-compactor: Sunday 9:13am cron. post-generator: weekly engineer log → posts.

**Runtime files** (gitignored): `Data/agent_logs/project_board.md`, `Data/agent_logs/engineer_log.md`, `Data/agent_logs/feature_tracker.md`.

**Bootstrap:** `/init` — generates CLAUDE.md + pm-config. Fill `vision.rules` + `posts.author` manually.
