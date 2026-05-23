---
name: Local Claude Code agents
description: project-vision, devtrack-engineer, git-agent, memory-compactor, post-generator — roles and board format
type: project
---

Five agents in `.claude/agents/`. Always invoke `project-vision` first; it creates board tasks and dispatches the engineer. Never invoke `devtrack-engineer` without a board task.

**Runtime files** (gitignored): `Data/agent_logs/project_board.md` (task board), `Data/agent_logs/engineer_log.md` (per-commit log).

**Core rules:** commits via `devtrack git commit` only; every task on a typed branch; board+log updated after every commit; PRs require developer approval before merge.

**Engineer log format:** `### [YYYY-MM-DD HH:MM] TASK-NNN — <what> / Original: "..." / Enhanced: "..." / Ticket: YES/NO / PM: YES/NO / Friction: LOW|MED|HIGH`

**git-agent:** pure plumbing — branches, push, merge to dev, PR (`--base dev`). Never commits code.

**memory-compactor:** Sunday 9:13am cron; compacts both `.claude/memory/` dirs; no manual action needed.

**post-generator:** weekly — engineer log → dev.to / HN / LinkedIn posts.
