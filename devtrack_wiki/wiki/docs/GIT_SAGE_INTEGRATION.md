# Using git-sage

git-sage is the Go-native AI git agent bundled with the `devtrack` binary. It understands repository context, can execute approved git operations, resolve suitable conflicts, and answer git questions in plain English. Its public entry point is `devtrack sage`.

git-sage is client-owned — it runs on your machine alongside the `devtrack` binary, using your local Ollama instance (or any configured LLM). No server required.

---

## Modes

### Ask Mode

Get explanations and command suggestions without executing anything:

```bash
devtrack sage ask "how do I undo my last commit but keep the changes"
devtrack sage ask "what's the difference between rebase and merge"
devtrack sage ask "how do I squash my last 5 commits"
```

### Do Mode (Agentic)

Execute git operations autonomously. git-sage plans the steps, runs them, reads the output, and handles failures:

```bash
devtrack sage do "squash my last 5 commits into one with a clean message"
devtrack sage do "merge feature-auth into main and resolve any conflicts"
devtrack sage do "my last push broke things, revert to the previous state"
devtrack sage do "clean up my branch — rebase onto main, fix conflicts"
```

Before the first command executes, you'll see an approval dialog with three options:
- **auto** — run all steps without interruption
- **review** — confirm each step before it runs
- **suggest-only** — plan only, no execution

### Interactive Shell

```bash
devtrack sage
sage> do merge feature-x into main
sage> ask what is cherry-pick
sage> context
sage> undo
sage> quit
```

Use `undo [N]` to roll back the last N steps. Up to 5 follow-up questions or tasks can be asked in the same session context.

---

## Configuration

Key environment variables are read by the embedded agent:

| Variable | Description |
|---|---|
| `GIT_SAGE_DEFAULT_MODEL` | Model for git-sage (e.g. `llama3`, `mistral`) |
| `OLLAMA_HOST` | Ollama server URL |
| `GIT_SAGE_PROVIDER` | LLM provider (`ollama`, `openai`, `groq`) |
| `GROQ_API_KEY` | Groq API key (if using Groq) |
| `GROQ_MODEL` | Groq model name |

---

## Integration with DevTrack

git-sage is automatically invoked by the DevTrack daemon for:

### Commit Enhancement

When you run `devtrack git commit`, git-sage analyzes your staged changes and the current git context (branch, recent commits, related PRs) to generate a well-structured commit message:

```bash
devtrack git commit -m "initial message"
# DevTrack enhances it using git-sage context before committing
```

### Conflict Auto-Resolution

After a merge or rebase that leaves conflicts, DevTrack can automatically invoke git-sage to resolve them. Conflicts that can be resolved safely are handled; those requiring human judgment are reported clearly.

Check logs to see what was resolved:

```bash
devtrack logs | grep -i conflict
```

### Work Update Context Enrichment

When the silent scheduler fires, DevTrack injects current branch, recent commit, and linked PR context into the enrichment pipeline without prompting in the main flow.

---

## How It Handles Failures

git-sage takes HEAD snapshots before each step. If a step fails:
1. It reads the error output and adjusts the plan
2. It attempts a recovery strategy (e.g. `git reset --soft HEAD~1`)
3. If it cannot recover, it reports which step failed and what the repo state is

It never uses `git rebase -i` (interactive editor blocks the agent loop). Squash operations always use `git reset --soft HEAD~N && git commit`.

---

## Troubleshooting

**Agent does nothing then says Done**

The LLM is returning prose instead of JSON. Check the model:
```bash
devtrack sage ask "hello"   # test with a simple request
```
Switch to a model with reliable JSON output: `llama3`, `mistral`, or `llama-3.3-70b-versatile` on Groq.

**Agent loops or takes too long**

```bash
# Interrupt with Ctrl+C, then check repo state
git status
git log --oneline -5
```

**Groq 403 error**

Make sure `openai` SDK is installed: `uv add openai` in the client directory. The Groq provider uses the OpenAI SDK to avoid Cloudflare blocks.

**Conflicts not resolving**

Some conflicts genuinely require human judgment — git-sage will tell you which files it could not resolve and why. Fix those manually, then `git add` and continue.

---

## Limitations

- git-sage does not push to remotes unless you explicitly ask it to (`devtrack sage do "push this branch"`)
- It will not force-push without being told explicitly
- It will not delete branches on remotes without explicit instruction
- Interactive rebase (`git rebase -i`) is not supported — use `devtrack sage do "squash my last N commits"` instead
