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
- **Interactive extras (v3.0, native Go):** an interactive terminal also gets a ticket picker (links the commit and appends a `Refs: <id>` trailer), a time-tracking prompt, immediate PM sync of a commit comment, and an auto-push prompt. PM posting uses the Go connectors (GitHub/GitLab/Azure); failures are queued to `pm_update_queue` and drained by the daemon when back online. All extras skip gracefully without PM credentials, on `pm_platform: none`, or with no open tickets.

### Caveat: reasoning models

Reasoning models (e.g. `qwen3`) stream `thinking` tokens before the answer; the shared 120s client timeout in `gitsage/llm.go` can elapse before the real `content` arrives, yielding an empty message and triggering the plain-commit fallback. Use a non-reasoning model (e.g. `gemma`) for reliable AI commits.

### Debugging checklist

- [ ] Verify the LLM provider is reachable (`curl $OLLAMA_HOST/api/tags`)
- [ ] Confirm `GIT_SAGE_DEFAULT_MODEL` names a model you actually have installed
- [ ] Test `devtrack git commit -m "message"` with staged changes
- [ ] Confirm fallback works: with the provider stopped, the commit still goes through with your message
