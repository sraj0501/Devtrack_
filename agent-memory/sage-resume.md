---
name: Sage fresh-session handoff
description: Exact resume context and remaining gates for the active Sage roadmap
type: project
---

# Sage fresh-session handoff

The project board owns task state and IDs. Recheck Git and GitHub because branch, PR, and CI state
can change after this record.

## Current checkpoint

- Active task: TASK-169 / SAGE-005 on `features/TASK-169-sage-mcp-readonly`.
- Implementation commit: `122c06e` (`feat(sage): add read-only MCP retrieval`).
- Integration base: `origin/dev` at `1b87ee6`, after merged PR #290. PRs #279–#290 are integrated.
- Implemented MCP tools: `sage_search`, `sage_topics`, `sage_get_entry`, and `sage_status`, alongside
  the existing six tools. All are read-only, non-destructive, idempotent, and closed-world.
- Search/topics/exact retrieval refresh from current deterministic Markdown. Status exposes bounded
  capture/pipeline/adapter state, not raw activity or local adapter paths.
- Local evidence: full Go suite and vet in Go 1.24 Linux; focused MCP/database race tests; native
  Windows MCP, Sage database, and main-package registration tests.
- PR #291 targets `dev`; all 19 hosted checks pass on head `8ba7dca`. Integration remains and must
  not be performed without explicit authorization.
- Next approved task: TASK-170 operational closure after TASK-169 integration.
- Reference: `C:\Work\git_apps\ai_sessions_skills` at authoritative clean HEAD `ea2e067`.

## Resume commands

```text
git status --short --branch
git log -6 --oneline --decorate origin/dev
python scripts/check_sage_parity.py --require-complete
python scripts/check_agent_memory.py
```

This machine has no host Go. Use the documented Docker Go 1.24 workflow; use cross-compiled native
Windows test binaries for Windows qualification. Keep host Ollama outside Docker unless a test
specifically needs an isolated endpoint.

## Scope boundaries

- Sage production is Go-native; Python is development tooling only.
- Hooks are silent, bounded, fail-open, and detached from model/network/database availability.
- SQLite owns runtime state; deterministic Markdown is the user-visible knowledge artifact.
- MCP remains local read-only stdio and excludes raw events, logs, prompts, output, reasoning, and
  adapter configuration paths.
- Sage may commit only Sage-owned knowledge files locally and never pushes.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
