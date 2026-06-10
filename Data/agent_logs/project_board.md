# DevTrack Project Board

_Last updated: 2026-06-10 by PM_
_Next DevTrack task ID: TASK-057_
_Active branch: `dev`_
_Shipped: v3.0.9 (2026-06-09) — skip_issues dual-platform fix._
_Direction: **PRODUCT_BIBLE.md** (pivot 2026-06-10) — `../../PRODUCT_BIBLE.md`_

---

## NORTH STAR

DevTrack is a silent background AI layer that absorbs developer meta-work — ticket
updates, EOD reports, PR review cycles, time tracking — by watching commits and
inferring the rest. The developer's only obligation: name branches with ticket IDs.

> **You write code. DevTrack handles the rest — silently, accurately, in your voice,
> getting better every day.**

Build arc is sequenced in trust order: **safe → accurate → automated → autonomous.**
Phases are defined in `PRODUCT_BIBLE.md` § Build Phases. Do not skip phases. Each
phase is a usable, testable increment with an explicit exit criterion.

---

## ACTIVE — Phase 0: Foundation reset

**Goal**: Remove TUI prompts from the timer-trigger and commit-trigger flows. These
become fully silent. The daemon no longer asks anything during normal operation.
Existing PM sync, LLM pipeline, and git monitor remain untouched.

**Exit criterion**: Daemon runs for a full day with no prompts shown.

**Status**: NOT STARTED — next up. Tasks to be decomposed by project-vision.

---

## QUEUED — Phases 1–8

| Phase | Name | Exit criterion (short) |
|---|---|---|
| 1 | Pending actions queue | A week of outbound actions all staged in `pending_actions`; nothing unexpected posts |
| 2 | Opinionated ticket extractor | >80% of commits mapped to tickets with no config beyond branch naming |
| 3 | Silent commit handler | Commit → ticket commented + state-transitioned within auto-approve window; dev did nothing |
| 4 | EOD pipeline | Accurate EOD email every evening, reads like the dev wrote it |
| 5 | Voice training (low friction) | After 1 week, generated text passes the "did I write this?" test |
| 6 | Dialectic self-improvement | After 30 days, correction rate measurably down; ≥3 autonomous skills emerged |
| 7 | TUI as visibility + correction | Open TUI → understand last 24h + everything about to happen |
| 8 | PR review loop (puppet master) | Push PR with nit comments, get "approved" without touching it again |

Full phase specs and acceptance criteria: `PRODUCT_BIBLE.md` § Build Phases.

---

## DEPRIORITISED (pivot 2026-06-10)

These sat on the old v3.x "Polish & Growth" board. The pivot moved them below the
Product Bible phases — not cancelled, just not now.

- ~~TASK-052~~ CLI aesthetics & theming (lipgloss) — deprioritised
- ~~TASK-053~~ "Work you didn't write" savings counter — deprioritised
- ~~TASK-054~~ How-to video series — deprioritised (revisit after Phase 4)
- PG-5 (`stats_client.py` → `GET /internal/stats`) — deprioritised
- Redis R-1 → R-6 — deprioritised
- Boardroom / plan as **primary** features — demoted to secondary (shipped & maintained, not a headline)

---

## SHIPPED — history (compact)

### v3.x line (2026-05 → 2026-06)
| Version | What |
|---|---|
| v3.0.9 | TASK-056 — `skip_issues` flag; dual-platform duplicate-ticket fix |
| v3.0.8 | Stale health snapshot fix; migration 005 prunes legacy Redis/MongoDB rows |
| v3.0.7 | Automated GitHub Actions release pipeline (`release.yml`) |
| v3.0.6 | Windows CLI full parity + autostart via Task Scheduler |
| v3.0.1–5 | `motor` optional dep; `upgrade.go` → GitHub API; Telegram bot migrated to Go |
| v3.0.0 | Go package refactor + client-standalone (TASK-A–F; TASK-E/F layered refactor) |

### Client-Server Decoupling
- **Phase 1 (1a–1d)** — server-mgmt removed from client; reports/learning/auth/license → HTTP; `workspaces.yaml` sole non-secret PM config source.
- **Phase 2 (TASK-055)** — native Go alert poller (`internal/alerts/`) + notifiers (`internal/notify/`) + interactive Telegram bot (`internal/telegram/`); daemon no longer spawns Python telegram/alert subprocesses. (commit `abad449`, follow-ups `e993507`/`c74179f`/`d4d9b5f`)

### EPIC-SPLIT (2026-05-24) — TASK-041–048
Monorepo restructured into `devtrack_client/` + `devtrack_server/` + `devtrack_wiki/`
with an HTTP/JSON boundary. Legacy `devtrack-bin/` + root `backend/` retired
(281 files, 69k lines).

### Earlier — TASK-000–040
v1.0.0 release + local agents; config audit (os.getenv eliminated across 22 files,
50+ accessors); CS-2 headless tests + server-TUI stats panel; CS-3 Admin GUI MVP
(users/licenses/health); logo + Windows binary icon; boardroom + plan commands.

---

_Append new active tasks under ACTIVE/QUEUED. Move completed work to SHIPPED as a
one-line entry — keep this board lean. Detailed per-task records live in
`feature_tracker.md` and `engineer_log.md`._
