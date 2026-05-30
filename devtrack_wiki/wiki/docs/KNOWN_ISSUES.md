# Known Issues & Debugging Notes

## AI Commit Enhancement (Go-native)

**Status:** Resolved — reimplemented in Go (v3.0)

As of v3.0, `devtrack git commit` is handled **natively in Go** (`devtrack_client/gitsage/commit.go`). The legacy `devtrack-git-wrapper.sh` shell wrapper — which shelled out to the Python backend and relied on a fragile stdout `grep -q "enhanced"` check — has been **removed**, along with the wrapper-only scripts `log_work.py` and `ticket_picker.py`. The old stdout/stderr mismatch failure mode no longer applies.

### Current behaviour

- `devtrack git commit [-m msg]` generates a Conventional-Commits message from the staged diff using the same LLM provider config as `devtrack sage` (`SAGE_PROVIDER` / `GIT_SAGE_PROVIDER`, model via `GIT_SAGE_DEFAULT_MODEL` / `SAGE_MODEL` / `GROQ_MODEL`).
- Interactive **A**ccept / **E**nhance / **R**egenerate / **C**ancel loop; auto-accepts when stdin is not a TTY (piped / CI).
- `--dry-run` previews the message without committing; `--no-enhance` commits your message as-is.
- `add`, `history`, and all other git subcommands pass straight through to `git`.
- **Graceful degradation:** if the LLM provider is unreachable, the commit proceeds with your message as-is (a standalone client has no server fallback).
- **Interactive extras (v3.0, native Go):** an interactive terminal also gets a ticket picker (links the commit and appends a `Refs: <id>` trailer), a time-tracking prompt, immediate PM sync of a commit comment, and an auto-push prompt. The picker's `n` key **creates a new ticket** (GitHub issue / GitLab issue / Azure work item) from the commit, and when no open ticket exists you'll be offered the same if `PM_CREATE_ON_NO_MATCH=true`. PM posting/creation uses the Go connectors (GitHub/GitLab/Azure); comment failures are queued to `pm_update_queue` and drained by the daemon when back online. All extras skip gracefully without PM credentials, on `pm_platform: none`, or with no open tickets.
- **Smart ticket matching (v3.0, native Go):** tickets are loaded offline-first from the local `ticket_cache` (the live API refreshes it on success and is the fallback only when offline). Each candidate is scored against the commit signal — branch name, commit subject, staged file paths, and **any `#N` / `AB#N` reference in the commit message** — using offline fuzzy similarity (token cosine + Jaro-Winkler, with an exact ticket-number match from the branch or message dominating). The picker lists tickets most-likely first, shows each match's likelihood as a `NN%` prefix, and pre-selects the top match. Set `PM_MATCH_THRESHOLD` (a `0..1` fraction or `0..100` percentage) to auto-link the top match without opening the picker when it clears the bar. Matching is **hybrid**: set `PM_MATCH_EMBED_MODEL` (e.g. `nomic-embed-text`, served by `OLLAMA_HOST`) to blend in semantic similarity; without it, matching stays fully offline.
- **Structured PM comment (v3.0, native Go):** the posted commit update mirrors the legacy `log_work.py` format — **Commit** (short hash), **Author**, **Message**, and optional **Time spent** and **Status** lines. Status is derived from GitHub-style closing keywords in the message (`fix`/`close`/`resolve` → `done`, otherwise `in_progress`) and is shown only when not the default. Auto-created issues honour the workspace's `pm_milestone` (GitHub milestone number / GitLab milestone id).

### Deferred commits (offline-first AI enhancement)

When the AI provider is **unreachable** at commit time, the interactive flow (and the explicit `[Q]ueue for later` option) lets you **queue** the change instead of committing a plain message. The staged diff, branch, files, and message are stored in the `deferred_commits` outbox and the work is **left staged, not committed** (the Python-exact hold-diff model).

The queue is drained automatically when the LLM is back:

- **Daemon scheduler** retries enhancement every 30 minutes (so a reachable LLM clears the backlog by end of day).
- **Pre-push hook** runs `devtrack commits enhance` right before a push — matching "enhance when a push was supposed to be done". It always exits 0 and never blocks the push (set `DEVTRACK_NO_PREPUSH=1` to skip).

Enhanced commits move to a "ready for review" state. Apply them with:

```bash
devtrack commits pending   # list queued + enhanced commits
devtrack commits enhance   # manually retry AI enhancement now
devtrack commits review    # [A]ccept enhanced / [O]riginal / [S]kip / [D]iscard → applies the patch + commits
```

Old queued commits expire after `DEFERRED_COMMIT_EXPIRY_HOURS`.

**Durable, drift-proof storage.** Queuing captures the staged content as a content-addressed git object — a dangling commit pinned by `refs/devtrack/deferred/<id>` — so the work survives garbage collection and any later working-tree changes. At apply time the change is landed with a **3-way merge** (not a raw `git apply`), so it succeeds even when the surrounding code has drifted; genuine overlaps become standard conflict markers. If auto-apply can't proceed, the pinned snapshot is offered for manual recovery (`git cherry-pick <snapshot>`), so a queued change can never be lost.

### Caveat: reasoning models

Reasoning models (e.g. `qwen3`) stream `thinking` tokens before the answer; the shared 120s client timeout in `gitsage/llm.go` can elapse before the real `content` arrives, yielding an empty message and triggering the plain-commit fallback. Use a non-reasoning model (e.g. `gemma`) for reliable AI commits.

### Debugging checklist

- [ ] Verify the LLM provider is reachable (`curl $OLLAMA_HOST/api/tags`)
- [ ] Confirm `GIT_SAGE_DEFAULT_MODEL` names a model you actually have installed
- [ ] Test `devtrack git commit -m "message"` with staged changes
- [ ] Confirm fallback works: with the provider stopped, the commit still goes through with your message
