# DevTrack Sage and the retired Git Sage name

DevTrack Sage is the Go-native, local command-knowledge subsystem bundled with `devtrack`. It
captures privacy-minimized command activity from supported coding harnesses, imports it into local
SQLite, and makes repeated command knowledge searchable. It is not a repository chat agent and it
does not execute Git operations.

Older documentation used **Git Sage** for a different repository-agent experiment. That surface is
retired: `sage ask`, `sage do`, `sage pr`, `sage interactive`, `sage git`, and free-form questions
no longer exist. Do not use `GIT_SAGE_*` settings for current Sage; a few Python-server reads remain
only as legacy model fallbacks. The independently useful enhanced-commit workflow remains available
as `devtrack git`, while `devtrack sage` now means command capture and personal knowledge.

| Need | Current command |
|---|---|
| Commit-message assistance or Git pass-through | `devtrack git ...` |
| Silent command capture and local knowledge search | `devtrack sage ...` |
| Repository questions or autonomous Git actions | Not provided |

## Install capture

```bash
devtrack sage harness list
devtrack sage harness install codex
devtrack sage status
```

The installed hook is silent, bounded, model-free, and fail-open. It omits raw command output,
reasoning, messages, and working-directory paths. On Windows, Codex uses an opt-in read-only history
adapter to avoid console-host compatibility problems.

Pause or remove capture without changing unrelated harness configuration:

```bash
devtrack sage pause
devtrack sage resume
devtrack sage harness uninstall codex
```

## Search local knowledge

```bash
devtrack sage search "rebase"
devtrack sage search "status command" --topic git
devtrack sage topics
devtrack sage doctor
```

The current development milestone includes capture, deterministic signatures, grouping, source
attribution, topics, and SQLite FTS search. Structured local-model distillation, deterministic
Markdown topic files, correction-stable routing, merge/refile, retries, and safe local knowledge
commits remain in progress until SAGE-003 closes.

## Git workflows

Use normal Git commands for repository operations. `devtrack git commit` remains an explicit,
separate enhanced-commit workflow and uses the shared `LLM_PROVIDER`, `OLLAMA_HOST`, and
`OLLAMA_MODEL` settings. Removed `sage ask`, `sage do`, `sage pr`, `sage interactive`, `sage git`,
and free-form Sage questions fail clearly instead of starting a model or Git operation.
