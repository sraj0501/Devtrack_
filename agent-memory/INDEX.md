# DevTrack Shared Agent Memory

_Last updated: 2026-09-21_ | active work only | GitHub is canonical

DevTrack is an offline-first silent Go daemon with an optional Python AI/server layer.

## Read first

- This directory is the only canonical location for shared agent memory. Agent-specific files are
  discovery adapters only and must not contain durable project state.
- `PRODUCT_BIBLE.md` is definitive product direction; [operations/build-guide.md](operations/build-guide.md)
  is the shared build guide.
- `Data/agent_logs/project_board.md` is the task/status/ID authority; dated notes are historical.
- This memory contains only durable rules, active work, and unresolved follow-ups. Do not reconstruct closed work from old plans or validation evidence.

## Active work

- [current-state.md](current-state.md) — current initiatives, branch state, and unresolved release work.
- [execution-plan.md](execution-plan.md) — ordered pickup sequence and acceptance gates for the active work.
- [initiatives/sage.md](initiatives/sage.md) — active DevTrack Sage port of the cross-harness, self-writing command-knowledge product; legacy Git Sage and playback are excluded.
- [initiatives/server-ui.md](initiatives/server-ui.md) — active admin-console visual work.

## Unresolved release follow-ups

- Qualify the packaged build rather than a source checkout.
- Capture and approve privacy-reviewed media from real behavior.
- Record the exact Glama listing path and update the score badge on awesome-mcp-servers PR #13608; do not guess the normalized slug.

## Rules and durable context

- [project-config.md](project-config.md) — shared paths, Git workflow, tests, scans, and project defaults.
- [engineering-rules.md](engineering-rules.md) — authorization, Git, privacy, architecture, and dependency rules.
- [operations/build-guide.md](operations/build-guide.md) — shared repository build and architecture guide.
- [components/client.md](components/client.md) and [components/server.md](components/server.md) — component-specific build guidance.
- [operations/platform-modes.md](operations/platform-modes.md) — mode resolution and cross-platform host guidance.
- [operations/environment-loading.md](operations/environment-loading.md) — environment resolution and test isolation.
- [operations/documentation.md](operations/documentation.md) — documentation sources and stale-claim rules.
- [operations/agent-system.md](operations/agent-system.md) — adapter boundary, canonical memory, and authorization.
- [operations/saas-license.md](operations/saas-license.md) — local license/auth and hosted-product boundary.
- [operations/mcp-distribution.md](operations/mcp-distribution.md) — MCP packaging rules and unresolved Glama action.
- [operations/launch-strategy.md](operations/launch-strategy.md) — positioning and publication boundary.
- [operations/reference-subsystems.md](operations/reference-subsystems.md) — Telegram, RAG, and Azure DevOps configuration.

## Tool-neutral role playbooks

These roles are available to Claude, Codex, Copilot, Cursor, Devin, OpenCode, and any other agent
working in this repository. Read a playbook when the user explicitly requests that role or the task
directly matches it. A role never expands the user's authorization.

- [roles/project-vision.md](roles/project-vision.md) — plan approved work and maintain the project board.
- [roles/devtrack-engineer.md](roles/devtrack-engineer.md) — implement an approved board task.
- [roles/git-agent.md](roles/git-agent.md) — perform explicitly requested Git plumbing.
- [roles/docu-agent.md](roles/docu-agent.md) — synchronize the wiki, shared memory, and README.
- [roles/memory-compactor.md](roles/memory-compactor.md) — audit and compact shared project memory.
- [roles/post-generator.md](roles/post-generator.md) — draft evidence-based weekly posts without publishing them.
