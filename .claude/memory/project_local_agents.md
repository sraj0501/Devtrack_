---
name: Local Claude Code agents
description: project-vision (PM), devtrack-engineer, post-generator — PM workflow and board format
type: project
---

Three agents in `.claude/agents/`. Always invoke `project-vision` first — it creates the board task and dispatches the engineer. Never invoke `devtrack-engineer` without a board task.

**Runtime files** (gitignored):
- `Data/agent_logs/project_board.md` — task board (IN PROGRESS / PLANNED / DONE / BLOCKED)
- `Data/agent_logs/engineer_log.md` — per-commit log

**Core rules** (both agents enforce):
1. All commits via `devtrack git commit` — never raw `git commit`
2. Every task on a `features/TASK-NNN-*`, `fix/TASK-NNN-*`, or `docs/TASK-NNN-*` branch
3. Board + engineer log updated after every commit
4. PR on task completion — do NOT merge without developer approval

**Engineer log entry format**:
```
### [YYYY-MM-DD HH:MM] TASK-NNN — <what>
Original message: "..." / Enhanced: "..." / Ticket linked: YES/NO / PM updated: YES/NO / Friction: LOW|MEDIUM|HIGH
```

`post-generator`: invoke weekly — turns engineer log into dev.to article, HN Show HN, LinkedIn post.
