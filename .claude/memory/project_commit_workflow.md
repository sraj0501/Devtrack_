---
name: Commit Workflow Redesign
description: Planned redesign of the commit workflow — context-aware ticket ranking, two-tier cache, shadow branches, approval TUI, in-terminal PM updates
type: project
---

Full design in `docs/COMMIT_WORKFLOW_DESIGN.md`. Key decisions:

**Core principle**: AI ranks, human picks. AI reasons across all assigned tickets + diff context. Developer confirms from a ranked shortlist — never a blind auto-assignment.

**Two-tier data freshness**:
- Full ticket cache (local SQLite + embeddings) for bulk ranking — eventually consistent, background refresh every 6h
- Live fetch of top 5 candidates at commit time for guaranteed-fresh data on the tickets that matter
- Why: keeps ranking fast (local) while ensuring developer never approves against stale ticket data

**PM sync is mandatory at onboarding** — `devtrack init` does not complete without a successful ticket sync. DevTrack cannot operate without ticket context loaded.

**Shadow branches** — `devtrack/<branch-name>` captures every commit attempt locally as an audit trail. Never pushed automatically. Pruned after 30 days. Never blocks the real commit if it fails.

**Offline behaviour** — commit always goes through. PM updates queue in SQLite (`pm_update_queue` table) and fire on next `devtrack sync`.

**New ticket creation is in-flow** — developer can create a ticket from the approval TUI without leaving the terminal. AI drafts title/description/epic from diff context.

**Build order** (not yet started):
1. Ticket cache (SQLite schema + GitHub sync)
2. Ticket embedder (embed on sync)
3. Context ranker (diff embed + cosine score + branch/session rerank)
4. Live fetcher (top-5 fresh fetch)
5. Shadow branch manager
6. Approval TUI
7. New ticket creator (in-TUI)
8. Async PM updater + queue
9. Commit orchestrator (replaces commit_message_enhancer.py hook)

**What this replaces**: `backend/commit_message_enhancer.py` (hook-only, no ticket context). `backend/task_matcher.py` is extended (not replaced) as the ranker foundation.

**PM platform scope**: GitHub first. Jira/Azure slot in later via the same sync interface.
