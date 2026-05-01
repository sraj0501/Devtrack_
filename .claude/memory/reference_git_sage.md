---
name: git-sage Implementation Reference
description: git-sage session UX, env config, key files for the git agent subsystem
type: reference
---

## git-sage Session UX (current implementation)

### Session Approval Mode
At the start of every `devtrack sage do <task>` or entering interactive mode:
```
Session approval mode:
  [a]  Accept all    — run every command without asking
  [r]  Review each   — confirm every command before it runs
  [s]  Suggest only  — show commands, never execute them
```
- Mode persists for the session; `--yes` / `-y` flag skips dialog (auto)
- Interactive mode: approval asked once and applied to all `do` calls in that session

### Follow-up Loop (do mode)
- After each task completes, user can ask up to **5 follow-up questions**
- Same agent instance -> same conversation history -> full context retained
- Empty Enter exits; `history` / `undo [N]` work inline during follow-up
- Undo prompt shown once after all follow-ups finish

### Command History & Undo
- `GitAgent` captures `git HEAD` before every `run` action in `state.step_log`
- `get_step_history()` / `undo_step(idx)` -> `git reset --hard <pre-step-HEAD>`
- `offer_undo()` prints numbered table, prompts for step to undo
- Interactive mode: `history` and `undo [N]` commands available
- Limitation: only git state undone; `write_file` edits are NOT reverted

### Squash Fix
- System prompt explicitly bans `git rebase -i` for squashing
- Agent always uses 3-step flow: `git --no-pager log -N`, `git reset --soft HEAD~N`, `git commit -m "..."`

### JSON Mode Enforcement
- `raw_chat(..., json_mode=True)` on every agent call
- Ollama: `"format": "json"` in payload
- OpenAI/Groq: `response_format={"type": "json_object"}` with `BadRequestError` fallback
- Model names strip `provider/` prefix (`groq/compound` -> `compound-beta`) before API call

### Groq Support
- `GIT_SAGE_PROVIDER=groq` + `GROQ_API_KEY` in `.env`
- Uses openai SDK (not urllib) — avoids Cloudflare 403 block on `Python-urllib` UA
- Recommended model: `llama-3.3-70b-versatile` (better JSON compliance than `compound-beta`)

## Env-Driven Provider Config

| `.env` variable | Purpose |
|---|---|
| `GIT_SAGE_PROVIDER` | Provider override (defaults to `LLM_PROVIDER`) |
| `GIT_SAGE_DEFAULT_MODEL` | Model for ollama/lmstudio |
| `GIT_SAGE_API_KEY` | API key override (falls back to provider-specific key) |
| `GIT_SAGE_BASE_URL` | Base URL override |
| `GROQ_API_KEY` | Groq API key |
| `GROQ_HOST` | Groq endpoint (default: https://api.groq.com/openai/v1) |
| `GROQ_MODEL` | Groq model name (native, no `groq/` prefix needed) |

## Key Commits on sage-improvements

- `bc19020` feat: session approval mode + step history + undo
- `505811e` fix: replace rebase with reset --soft for squash
- `7b07343` fix: enforce JSON mode + show raw LLM output on parse errors
- `2ae4af0` fix: strip groq/ prefix + harden JSON-only enforcement
- `9e13aa2` fix: surface full answer for informational tasks in done.summary
- `0373de3` feat: follow-up loop in do mode (5 questions, same context)

## Key Files

```
backend/git_sage/
  cli.py       - Session approval, follow-up loop, history/undo, do_mode, interactive_mode
  agent.py     - _run_loop(), run(), followup(), suggest_only, step_log, undo_step()
  llm.py       - json_mode param, provider prefix stripping, BadRequestError fallback
  config.py    - Env-driven: .env > ~/.config/git-sage/config.json > defaults
  __main__.py  - Entry point for `python -m backend.git_sage`

devtrack-bin/
  main.go      - CLI entry; `sage` command delegates to runGitSage()
```
