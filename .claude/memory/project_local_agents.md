---
name: Agent system
description: Checked-in role sources, external-agent boundary, canonical memory, and authorization
type: project
---

**Checked-in sources:** `.claude/agents/_archive/` holds five archived Claude role definitions: project-vision, devtrack-engineer, git-agent, memory-compactor, and post-generator. There is no checked-in `.agents/` or `.codex/` adapter directory. `.claude/commands/docu-agent.md` is the checked-in documentation-maintenance workflow; the runtime used to execute it is environment-owned.
**Durable routing intent:** project-vision plans/creates approved tasks; devtrack-engineer implements an approved `TASK-NNN`; git-agent performs Git plumbing only; docu-agent changes docs/memory only; memory-compactor reconciles durable memory; post-generator requires engineer-log evidence. Repository `.claude/memory/` is canonical project memory; never read or modify user-level/external memory as if it were repository state.
**Authorization:** roles never imply permission to mutate the board, commit, push, open/merge a PR, publish, or deploy.
**Project records:** `Data/agent_logs/project_board.md`, `feature_tracker.md`, `engineer_log.md`, and selected post/evidence files are checked in and travel with the repository. Do not describe that directory as wholly gitignored. `.claude/pm-config.md` holds shared project values; transient tool/runtime activity outside these tracked files is not durable project memory.
