---
name: Agent system
description: 6 global agents at ~/.claude/agents/; pm-config.md drives project-specific values
type: project
---

**Global agents** at `~/.claude/agents/` (6): project-vision, engineer, git-agent, memory-compactor, post-generator, production-engineer. Same agents reused across all projects via `.claude/pm-config.md`.

**DevTrack pm-config:** `D:\git_apps\Devtrack_\.claude\pm-config.md`

**Invocation rules:**
- Always invoke `project-vision` first; it creates board tasks and dispatches the engineer.
- Never invoke `engineer` without a board task.
- Use `git-agent` for branch/push/PR (not raw commands). `git-agent` never commits code.
- `memory-compactor` runs Sunday 9:13am cron; compacts both `.claude/memory/` dirs.
- `post-generator` weekly: engineer log → dev.to / HN / LinkedIn posts.

**Runtime files** (gitignored): `Data/agent_logs/project_board.md`, `Data/agent_logs/engineer_log.md`.

**Engineer log format:** `### [YYYY-MM-DD HH:MM] TASK-NNN — <what> / Original: "..." / Enhanced: "..." / Ticket: YES/NO / PM: YES/NO / Friction: LOW|MED|HIGH`

**Bootstrap any new project:** run `/init` — generates CLAUDE.md + .claude/pm-config.md. Fill `vision.rules` and `posts.author` manually.
