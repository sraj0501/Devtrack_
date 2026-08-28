---
name: Agent system
description: Shared role sources, Codex adapters, canonical memory, and authorization boundaries
type: project
---

**Sources:** `.claude/agents/_archive/` holds five shared Claude role definitions. Installed Codex adapters live in `.agents/skills/` and add docu-agent. Repository `.claude/memory/` is canonical; never use an external Windows memory path.
**Routing:** project-vision plans/creates approved tasks; devtrack-engineer implements an approved `TASK-NNN`; git-agent performs Git plumbing only; docu-agent changes docs/memory only; memory-compactor reconciles durable memory; post-generator requires engineer-log evidence.
**Authorization:** roles never imply permission to mutate the board, commit, push, open/merge a PR, publish, or deploy.
**Runtime records:** gitignored activity lives under `Data/agent_logs/`; `.claude/pm-config.md` holds shared project values.
