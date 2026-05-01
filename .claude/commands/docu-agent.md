You are the DevTrack documentation agent. Your job is to keep all project documentation in sync with the current state of the codebase.

Run the following three sub-agents **in parallel** (launch all three in a single message with multiple Agent tool calls):

---

## Agent 1 — Wiki (`wiki/wiki.html`)

1. Run `GIT_NO_DEVTRACK=1 git log --oneline -20` to see what changed recently.
2. Read `wiki/wiki.html` to understand the current structure (single-file SPA with inline page sections, nav sidebar, and home grid cards).
3. For each new feature or change identified from git log:
   - Add a new inline page section (`<div class="content" id="PAGE_ID">`) if the feature warrants its own page
   - Add a nav entry in the appropriate group in the sidebar
   - Add a home grid card if it's a major feature
4. Update the WHATS_NEW page: prepend a new version section at the top for any unreleased changes. Preserve all existing content below.
5. Update the version chip and home badge if a version bump is warranted.

---

## Agent 2 — Memory (`/Users/sraj/.claude/projects/-Users-sraj-git-apps-personal-automation-tools/memory/`)

1. Run `GIT_NO_DEVTRACK=1 git log --oneline -20` to see what changed recently.
2. Read `MEMORY.md` to understand what's already recorded.
3. Update `MEMORY.md`:
   - Add a new "Completed" subsection for the session date with bullet points for each shipped feature
   - Move any items from "Planned" to "Completed" if they are now shipped
   - Update the **Project Status** line at the top
   - Update Key CLI Files list if new files were added
4. Create or update individual memory files (e.g. `project_*.md`) for significant new features that need detailed notes. Follow the frontmatter format: `name`, `description`, `type`, `---`, then content with **Why:** and **How to apply:** lines.
5. Add pointer lines to `MEMORY.md` index for any new memory files.

---

## Agent 3 — README (`README.md`)

1. Run `GIT_NO_DEVTRACK=1 git log --oneline -20` to see what changed recently.
2. Read `README.md` to understand current structure.
3. For each new major feature:
   - Add a row to any relevant feature/command tables
   - Add a concise subsection under the relevant heading (Setup, Core Features, CLI Reference, etc.)
   - Add rows to the documentation table pointing to new wiki pages or doc files
4. Keep additions concise — README is a quick-start reference, not a full manual.

---

## After all three agents complete

1. Run `GIT_NO_DEVTRACK=1 git status --short` to see which doc files changed.
2. Stage and commit only the documentation files with:
   ```
   GIT_NO_DEVTRACK=1 git add wiki/wiki.html README.md docs/ memory/
   GIT_NO_DEVTRACK=1 git commit -m "docs: <brief summary of what was documented>"
   ```
3. Push: `GIT_NO_DEVTRACK=1 git push origin main`

Do NOT commit or modify any source code files (.go, .py, etc.). Documentation only.
