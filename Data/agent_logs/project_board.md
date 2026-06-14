# DevTrack Project Board

_Last updated: 2026-06-14 by PM (Phase 0 decomposition)_
_Next DevTrack task ID: TASK-060_
_Active branch: `dev`_
_Shipped: v3.0.10 (2026-06-14) — significant Windows fixes + gitsage improvements._
_Direction: **PRODUCT_BIBLE.md** (pivot 2026-06-10) — `../../PRODUCT_BIBLE.md`_

**[2026-06-14] Ad-hoc session commit** — commit `0a9d9a7` on dev, pushed; PR #161 (dev → main) opened.
Three gitsage improvements: (1) Windows isatty fix via mattn/go-isatty; (2) editor-commit BeforeCommit/AfterCommit hooks; (3) background auto-enhance (opt-in DEVTRACK_AUTO_ENHANCE=true). Build/vet clean. PR: https://github.com/sraj0501/Devtrack_/pull/161

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

**Status**: DECOMPOSED — 3 tasks ready to dispatch.

---

### TASK-057 — Silence handleTrigger stdout in integrated.go
**Priority**: HIGH
**Phase**: Phase 0
**Depends on**: none
**Branch**: `fix/TASK-057-silence-handle-trigger`

**Spec**:
The function `handleTrigger` in `devtrack_client/internal/infra/integrated.go`
(lines 347–479) prints a full decorative banner to stdout on every commit and
timer trigger. This banner (15–20 `fmt.Print*` calls) violates PRODUCT_BIBLE.md
Non-Negotiable #1 ("no prompts in the main flow") and is the only active source
of terminal output in the trigger flow.

Changes required, **all inside `handleTrigger()`**:

1. Replace every `fmt.Print*` call with `log.Printf` / `log.Println` (which go
   to the log file, not the terminal). The information (commit hash, message,
   author, files, workspace, trigger count, interval) is still useful in the log;
   keep it as a structured log line.
2. The decorative separator lines (`strings.Repeat("═", 60)`) and "What happens
   next:" paragraph are redundant in the log — remove them entirely. One log line
   per trigger is enough.
3. The "Waiting for next event..." line must be removed entirely.
4. `strings` import may become unused after the change — remove it if so (run
   `go vet ./...` to verify). Other uses of `strings` in the same file
   (`strings.EqualFold`, `strings.TrimSpace`, `strings.Join`) must be checked
   before dropping the import.
5. The `TestIntegrated()` function lower in the file also contains `fmt.Print*`
   calls — those are in a dev-test helper, not the live trigger path. Leave them
   as-is; the function is never called in production.

After the change, `handleTrigger` must contain zero `fmt.Print*` calls.
The function must still: log the trigger type + key fields at `log.Printf` level,
persist the trigger record to SQLite, and send the HTTP trigger to the Python
server — all unchanged.

**Acceptance criteria**:
- [x] `grep -n "fmt\.Print" devtrack_client/internal/infra/integrated.go` returns
      only matches inside `TestIntegrated()` (line ~510 onward), zero matches in
      `handleTrigger`.
- [x] `go build ./...` passes with no errors from `devtrack_client/`.
- [x] `go vet ./...` passes clean.
- [ ] The daemon log (`Data/logs/daemon.log`) still shows commit/timer events as
      log lines when the daemon runs. _(runtime verification — pending developer test)_
- [ ] No terminal output appears when a commit fires while the daemon is running
      in the background. _(runtime verification — pending developer test)_

**Engineer status**: 3/5 criteria done — last commit: f0399d7 "fix(infra): Silence stdout output from handleTrigger function" — 2026-06-14 15:45
**PR**: https://github.com/sraj0501/Devtrack_/pull/163
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-14 15:50

---

### TASK-058 — Remove or gate user_prompt.py from trigger path (Python server)
**Priority**: MEDIUM
**Phase**: Phase 0
**Depends on**: TASK-057 (can be worked in parallel — different file, different codebase)
**Branch**: `fix/TASK-058-remove-user-prompt-trigger`

**Spec**:
`devtrack_server/backend/user_prompt.py` defines `DevTrackTUI` and
`prompt_work_update()`, which can block the process waiting for stdin input.
Current state: `TriggerProcessor.process_commit()` and `process_timer()` in
`webhook_server.py` do NOT call `user_prompt` — the trigger path is already clean.
However, the module still exists and could be accidentally re-introduced.

Changes required:

1. Search the entire `devtrack_server/backend/` tree for any remaining import of
   `user_prompt` or call to `DevTrackTUI` / `prompt_work_update` / `prompt_user`
   outside of test files and the `__main__` block in `user_prompt.py` itself.
   Command: `grep -rn "user_prompt\|DevTrackTUI\|prompt_work_update\|prompt_for_work_update" devtrack_server/backend/ --include="*.py"`
2. If any non-test file imports `user_prompt` for use in a trigger path, remove
   or replace that call with a `logger.info()`.
3. Add a module-level docstring to `user_prompt.py` (top of file, after the
   existing docstring) noting:
   `# STATUS: Legacy module. Not called from any trigger path as of Phase 0.`
   `# Safe to delete once the TUI correction interface (Phase 7) is implemented.`
   Do NOT delete the file — it will be repurposed for the Phase 7 TUI visibility
   interface.
4. Run `uv run pytest backend/tests/ -q` to confirm no tests regress.

**Acceptance criteria**:
- [x] `grep -rn "user_prompt\|DevTrackTUI\|prompt_work_update" devtrack_server/backend/ --include="*.py"` returns zero hits outside `user_prompt.py` itself and `test_user_prompt.py`.
- [x] `uv run pytest backend/tests/ -q` — 591 pass, 1 pre-existing failure (`test_ollama_host_returns_string`, `OLLAMA_HOST=0.0.0.0` in shell, documented in engineer log).
- [x] The module-level status comment is present at the top of `user_prompt.py`.

**Engineer status**: 3/3 criteria done — last commit: 6d269ef "feat(user_prompt): Remove legacy user prompt logic" — 2026-06-14 15:55

**COMPLETE** — ready for PM review — 2026-06-14 15:55

**PR**: https://github.com/sraj0501/Devtrack_/pull/164
**Blockers**: none

---

### TASK-059 — Verify Phase 0 exit criterion: daemon silent for a full session
**Priority**: HIGH
**Phase**: Phase 0
**Depends on**: TASK-057, TASK-058
**Branch**: `fix/TASK-059-phase0-verification`

**Spec**:
This is the verification and cleanup task that closes Phase 0. It produces no new
feature code — only a verification run, any small fixes found during verification,
and the board/log updates that mark Phase 0 done.

Steps:

1. Build the client binary from `devtrack_client/`: `go build -o devtrack .`
2. Start the daemon: `devtrack start`
3. Make at least 2 commits in a watched repo (can be empty commits:
   `git commit --allow-empty -m "phase0 test 1"`).
4. Wait for at least one timer trigger to fire (set `PROMPT_INTERVAL=1` in `.env`
   for the test, restore after).
5. Check `Data/logs/daemon.log`: confirm trigger events appear as log lines.
6. Check the terminal where the daemon was launched (or attached to): confirm
   zero trigger banners / prompts appear.
7. Run `grep -n "fmt\.Print" devtrack_client/internal/infra/integrated.go` and
   confirm no matches in `handleTrigger`.
8. Run the full hardcoded-values scan (PM responsibility, run before closing):
   ```
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]" devtrack_client/ --include="*.go" | grep -v "_test\|#\|config\|Get"
   grep -rn "os\.getenv\b" devtrack_server/backend/ --include="*.py" | grep -v "config\.py\|conftest\|test_"
   ```
9. Update `Data/agent_logs/feature_tracker.md` with Phase 0 completion entry.
10. Open a PR targeting `dev` with title "Phase 0: silent daemon trigger flows".

**Acceptance criteria**:
- [ ] Zero terminal output from daemon during normal commit/timer operation.
- [ ] `Data/logs/daemon.log` contains structured log lines for each trigger.
- [ ] Hardcoded-values scan is clean (no new violations).
- [ ] `go build ./...` and `go vet ./...` pass clean.
- [ ] PR opened targeting `dev` (never `main`).
- [ ] Feature tracker updated.

**Engineer status**: not started
**Blockers**: TASK-057 and TASK-058 must be complete

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
| v3.0.10 | Significant Windows fixes: isatty via mattn/go-isatty; editor-commit hooks; auto-enhance (`DEVTRACK_AUTO_ENHANCE=true`) |
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
