# Shared agent memory

`agent-memory/` is the only canonical location for durable project context shared by Claude,
Codex, Copilot, Cursor, Devin, OpenCode, and other coding agents.

Every agent starts with [INDEX.md](INDEX.md), then reads only the linked records relevant to its
task. Active state, decisions, project rules, initiatives, and reusable build guidance belong here.

Agent-specific files and directories are adapters only. They may tell a harness how to discover
this directory or invoke a workflow, but they must not contain a second copy of durable project
state. In particular, do not create project memory under `.claude/`, `.codex/`, `.agents/`,
`.cursor/`, `.github/`, `.gemini/`, `.windsurf/`, or user-level agent storage.

Historical task evidence remains in `Data/agent_logs/`; source-of-truth product and architecture
documents remain at their existing repository paths and are linked from the index instead of
duplicated here.

Tool-neutral role playbooks live in `roles/` and are linked from [INDEX.md](INDEX.md). They are
shared workflow instructions, not automatically installed commands or permission grants.
Shared project paths and workflow defaults live in [project-config.md](project-config.md), never in
a harness-specific directory.

Run `python scripts/check_agent_memory.py` after changing memory or agent adapters.
