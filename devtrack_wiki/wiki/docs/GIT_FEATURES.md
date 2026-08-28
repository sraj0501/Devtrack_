# Git features

## Silent observation

The daemon watches enabled repositories and reacts after commits. It does not prompt during normal
Git work. Ticket context is resolved from branch names, commit prefixes, trailers, explicit work
sessions, or recent context. Failure to resolve a ticket is logged and never blocks Git.

## Optional enhanced commit

```bash
devtrack git add <paths>
devtrack git commit -m "short message"
devtrack git commit -m "short message" --dry-run
devtrack git commit -m "short message" --no-enhance
```

This explicitly invoked Go-native workflow can refine the message, offer a ticket picker, capture
time, and offer a push. If the LLM is unavailable, the original message remains usable.

Configuration uses `GIT_SAGE_PROVIDER`, `GIT_SAGE_DEFAULT_MODEL`, `OLLAMA_HOST`, and the appropriate
provider credential. Server `LLM_PROVIDER` does not replace the client git-sage configuration.

## Deferred commits

```bash
devtrack commits pending
devtrack commits enhance
devtrack commits review
```

Deferred staged work is pinned by a Git reference and can be recovered even if the worktree changes.
The pre-push enhancement hook never blocks a push.

## Git-sage agent

```bash
devtrack sage ask "how do I undo the last commit but keep the files?"
devtrack sage do "squash the last three commits" --verbose
devtrack sage pr
devtrack sage interactive
```

Squash operations use a soft reset plus commit, never interactive rebase. Agent failures roll back
when possible and do not add a blocking dependency to normal Git commands.

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
