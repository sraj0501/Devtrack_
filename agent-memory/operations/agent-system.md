---
name: Agent system
description: Checked-in role sources, external-agent boundary, canonical memory, and authorization
type: project
---

**Checked-in sources:** `AGENTS.md`, `CLAUDE.md`, component `CLAUDE.md` files, and `.github/copilot-instructions.md` are discovery adapters only. `.claude/agents/_archive/` holds five archived role definitions, and `.claude/commands/docu-agent.md` is a workflow adapter. None is a project-memory store.
**Canonical memory:** repository `agent-memory/` is the only shared project-memory location. Every harness begins with `agent-memory/INDEX.md` and reads only the linked files relevant to its task. Never create durable state under `.claude/`, `.codex/`, `.agents/`, `.cursor/`, `.github/`, `.gemini/`, or another harness-specific directory. Never read or modify user-level/environment-owned memory as if it were repository state.
**Durable routing intent:** project-vision plans/creates approved tasks; devtrack-engineer implements an approved `TASK-NNN`; git-agent performs Git plumbing only; docu-agent changes docs/shared memory only; memory-compactor reconciles `agent-memory/`; post-generator requires engineer-log evidence.
**Authorization:** roles never imply permission to mutate the board, commit, push, open/merge a PR, publish, or deploy.
**Project records:** `Data/agent_logs/project_board.md`, `feature_tracker.md`, `engineer_log.md`, and selected post/evidence files are checked in and travel with the repository. Do not describe that directory as wholly gitignored. `.claude/pm-config.md` holds shared project values; transient tool/runtime activity outside these tracked files is not durable project memory.
