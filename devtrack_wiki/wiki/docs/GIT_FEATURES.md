# Git features

> The silent native-Git behavior below is merged into `dev` in PR #264 but remains unreleased.
> Public v3.1.1 still has a known regression in its shell-wrapped commit path.

## Silent observation

The daemon watches enabled repositories and reacts after commits. It does not prompt during normal
Git work. The current configured ticket resolver runs in the background; failure to resolve a
ticket is recorded and never blocks Git. The stricter canonical branch and provenance contract is
planned work and is not presented here as shipped behavior.

## Optional enhanced commit

```bash
devtrack git add <paths>
devtrack git commit -m "short message"
devtrack git commit -m "short message" --dry-run
devtrack git commit -m "short message" --no-enhance
```

This explicitly invoked Go-native workflow can refine the message before Git runs. After the commit
it returns immediately: it does not offer a ticket picker, ask for time, post to a PM system, or
offer a push. The daemon observes the completed commit asynchronously and stages applicable work.
If the LLM is unavailable, the original message remains usable.

Configuration uses the shared `LLM_PROVIDER`, `OLLAMA_HOST`, `OLLAMA_MODEL`, and the appropriate
provider credential. Ollama remains the offline-first default.

## Deferred commits

```bash
devtrack commits pending
devtrack commits enhance
devtrack commits review
```

Deferred staged work is pinned by a Git reference and can be recovered even if the worktree changes.
The pre-push enhancement hook never blocks a push.

## DevTrack Sage command knowledge

```bash
devtrack sage harness install codex
devtrack sage status
devtrack sage search "undo commit"
devtrack sage topics
```

Sage silently captures a bounded, privacy-minimized command event from supported harnesses and
indexes it locally. It does not execute Git operations or answer repository questions; use normal
Git commands and `devtrack git` for explicit commit workflow assistance.

## Ticket and PM behavior

The Go connectors support GitHub, GitLab, and Azure DevOps issue operations. PM, email, and Git writes
are staged before execution. Use `skip_issues: true` for a code-only workspace when the same
repository has a separate PM-authoritative workspace.

## Troubleshooting

```bash
devtrack status
devtrack doctor
devtrack logs -f
curl "$OLLAMA_HOST/api/tags"
```

Use a generation-capable model and increase the configured long timeout for a healthy slow local
model.
