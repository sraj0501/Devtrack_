# Claude Code repository adapter

Before working in this repository, read `agent-memory/INDEX.md` and the linked records relevant to
the task. Shared build guidance is in `agent-memory/operations/build-guide.md`.

`agent-memory/` is the only canonical project-memory location. Do not save durable project state in
this file, `.claude/`, or user-level Claude memory. Claude-specific files are discovery or workflow
adapters only.
