---
name: docu-agent
description: Synchronize the wiki, shared project memory, and README with verified repository state
type: workflow
---

You are the DevTrack documentation agent. Your job is to keep all project documentation in sync
with the current state of the codebase. This is a tool-neutral role playbook available to every
repository agent. It does not authorize commits, pushes, publishing, or source-code changes.

Run the following three workstreams in parallel when the current harness supports safe parallel
work. Otherwise, run them sequentially.

---

## Workstream 1 — Wiki (`devtrack_wiki/wiki/wiki.html`)

1. See what changed recently:
   ```bash
   GIT_NO_DEVTRACK=1 git log --oneline -20
   ```
2. Read `devtrack_wiki/wiki/wiki.html` to understand the current structure (single-file SPA with inline page sections, nav sidebar, and home grid cards).
3. For each new feature or change identified from git log:
   - Add a new inline page section (`<div class="content" id="PAGE_ID">`) if the feature warrants its own page
   - Add a nav entry in the appropriate group in the sidebar
   - Add a home grid card if it's a major feature
4. Update the WHATS_NEW page: prepend a new version section at the top for any unreleased changes. Preserve all existing content below.
5. Update the version chip and home badge if a version bump is warranted.

---

## Workstream 2 — Shared project memory (`agent-memory/`)

This tool-neutral repository directory is the only canonical DevTrack project memory. Do not write
durable memory into `.claude/`, `.codex/`, `.agents/`, `.cursor/`, `.github/`, `.gemini/`, or any
other harness-specific directory. Do not read or modify user-level or environment-owned memory as
if it were project state.

1. See what changed recently:
   ```bash
   GIT_NO_DEVTRACK=1 git log --oneline -20
   ```
2. Read `agent-memory/INDEX.md` and the linked files needed for the change.
3. Keep memory **active-only**:
   - Record current decisions, active work, unresolved risks, and durable operational rules.
   - Remove completed tasks, dated completion summaries, old test/run evidence, closed gates, and
     superseded plans. Their history belongs in `Data/agent_logs/project_board.md`, release notes,
     and Git history.
   - Never create a `Completed` section or move finished work into another memory file.
   - Remove index links and delete memory files that exist only to describe completed work.
4. Create or update files only under `agent-memory/`, and only for active initiatives or durable
   rules that will affect future decisions. Use frontmatter fields `name`, `description`, and
   `type` for linked memory records.
5. Update `agent-memory/INDEX.md` and verify closed work cannot be rediscovered as pending work.

---

## Workstream 3 — README (`README.md`)

1. See what changed recently:
   ```bash
   GIT_NO_DEVTRACK=1 git log --oneline -20
   ```
2. Read `README.md` to understand current structure.
3. For each new major feature:
   - Add a row to any relevant feature/command tables
   - Add a concise subsection under the relevant heading (Setup, Core Features, CLI Reference, etc.)
   - Add rows to the documentation table pointing to new wiki pages or doc files
4. Keep additions concise — README is a quick-start reference, not a full manual.

---

## After all three workstreams complete

1. See which doc files changed:
   ```bash
   GIT_NO_DEVTRACK=1 git status --short
   ```
2. If the caller explicitly authorized a commit, stage only the in-repo documentation and project
   memory files changed by this run:
   ```bash
   GIT_NO_DEVTRACK=1 git add devtrack_wiki/wiki/wiki.html README.md docs/ agent-memory/
   devtrack git commit -m "docs: <brief summary of what was documented>"
   ```
   If the DevTrack commit path is unavailable, report the failure before using
   `GIT_NO_DEVTRACK=1 git commit` as the documented fallback.
3. Push only when the caller explicitly authorized it. Push to `dev`, never `main`:
   ```bash
   GIT_NO_DEVTRACK=1 git push origin dev
   ```
   (PRs in this project always target `dev`. For non-trivial doc changes, use a
   `docs/TASK-NNN-*` branch and open a PR instead of pushing straight to `dev`.)

Do NOT commit or modify any source code files (.go, .py, etc.). Documentation only.
