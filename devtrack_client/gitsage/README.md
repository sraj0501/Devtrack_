# git-sage

`gitsage/` is the Go-native, client-owned Git agent embedded in the `devtrack` binary. It has no
Python package, standalone `git-sage` executable, or `pip` installation path.

## Commands

```bash
devtrack sage ask "how do I undo the last commit but keep the changes?"
devtrack sage do "squash the last five commits" --verbose
devtrack sage pr
devtrack sage interactive
```

The `do` loop plans operations, executes them, observes output, and rolls back after failure when
possible. Approval modes are `auto`, `review`, and `suggest-only`. Follow-up context is retained for
up to five questions, and `undo [N]` reverts recorded steps.

## Configuration

| Variable | Purpose |
|---|---|
| `GIT_SAGE_PROVIDER` | `ollama`, `openai`, or `groq` |
| `GIT_SAGE_DEFAULT_MODEL` | Provider-native model name |
| `OLLAMA_HOST` | Local Ollama endpoint |
| `OPENAI_API_KEY` / `GROQ_API_KEY` | Optional cloud credentials |

Ollama is the default. Context stays local unless an external provider is explicitly configured.

## Safety behavior

- Normal Git remains usable when the LLM is unavailable.
- Commands are parsed from structured JSON; prose-only model output is rejected.
- Destructive ambiguity is surfaced for review.
- Squash uses `git reset --soft HEAD~N && git commit`; interactive rebase is never used.
- Conflict handling inspects markers and escalates when intent cannot be determined safely.

## Package map

| File | Role |
|---|---|
| `agent.go` | Agent loop and structured steps |
| `cli.go` | Command and approval-mode handling |
| `llm.go` / `config.go` | Provider calls and configuration |
| `context.go` | Repository context collection |
| `git_ops.go` | Structured Git operations and step history |
| `conflict.go` | Conflict inspection and resolution |
| `pr_finder.go` | Pull-request context |
| `commit.go` | AI-enhanced `devtrack git commit` flow |
