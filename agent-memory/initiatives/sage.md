---
name: DevTrack Sage implementation plan
description: Active completion plan and durable boundaries for local cross-harness command knowledge
type: project
---

DevTrack Sage is the local, Go-native, cross-harness command-knowledge product. The detailed plan
is `docs/DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`; the executable parity authority is
`docs/SAGE_PORT_PARITY_MATRIX.md`.

## Active state

PRs #279–#290 are integrated into `dev` at `1b87ee6`. The 136-row reference ledger is complete at
119 implemented and 17 adapted, and the clean capture-to-knowledge journey passes repeatedly on
Windows and Linux. Codex, Claude Code, GitHub Copilot CLI, OpenCode, Cursor, and Devin CLI are
independent advertised adapters.

TASK-169 is fully green on PR #291 at head `bf963dd`; integration into `dev` remains. TASK-170 is
active on `features/TASK-170-sage-operational-closure`, stacked from that head. It implements
dry-run/apply retention, bounded opaque quarantine inspection and confirmed purge, confirmed exact
topic deletion, deterministic Markdown/index/cache repair, and validated additive Markdown
export/import. Linux full tests/vet, focused race/fault coverage, and native Windows focused tests
and both documentation reconciliation passes pass locally; commit/push, the dedicated PR, and
final-head hosted checks remain.

## Durable product surface

```text
devtrack sage status|doctor|log
devtrack sage pause|resume|restart
devtrack sage search <query> [--topic <topic>]
devtrack sage topics
devtrack sage routes|route|merge
devtrack sage retention|quarantine|delete-topic|repair|export|import
devtrack sage harness list|install|uninstall
```

The local MCP development surface adds `sage_search`, `sage_topics`, `sage_get_entry`, and
`sage_status`. The retired repository-chat/Git-operation Sage commands must not return.

## Durable invariants

- Port behavioral parity from `C:\Work\git_apps\ai_sessions_skills` at authoritative commit
  `ea2e067`; do not copy Python process boundaries into production.
- Hooks normalize and atomically spool bounded events. They never open SQLite or wait for a model,
  Python, or a network service.
- SQLite owns runtime queues and derived indexes. Deterministic Markdown is the portable,
  user-visible artifact and the authority for retrieval reconciliation.
- Normal capture excludes raw output, user messages, reasoning, and private argument values where
  possible. MCP never exposes raw events, logs, prompts, output, reasoning, or config paths.
- Model failures remain retryable. Sage may commit only validated Sage-owned knowledge locally and
  never pushes.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  non-goals.
