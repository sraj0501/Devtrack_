# Git features

> The silent native-Git behavior below ships in v3.2.0.

## Silent observation

The daemon watches enabled repositories and reacts after commits. It does not prompt during normal
Git work. The current configured ticket resolver runs in the background; failure to resolve a
ticket is recorded and never blocks Git. Canonical branch rules, mapping provenance, and immutable
correction audit are part of the shipped deterministic ticket resolver.

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
devtrack sage harness install claude
devtrack sage harness install copilot
devtrack sage harness install opencode
devtrack sage status
devtrack sage search "undo commit"
devtrack sage topics
```

Sage silently captures a bounded, privacy-minimized command event from supported harnesses and
indexes it locally. It does not execute Git operations or answer repository questions; use normal
Git commands and `devtrack git` for explicit commit workflow assistance.

## Ticket and PM behavior

An incident, story, bug, or task is identified by the configured workspace's canonical reference;
the PM system owns its item type. For example, `hotfix/OPS-123-restore-login` can map to a Jira
incident, while `feature/ADO-456-add-export` can map to an Azure DevOps story. The branch kind
does not determine the PM type. Full-branch evidence wins over explicit commit prefix/trailer,
which wins over selected ticket context. Conflicting evidence is recorded for review; incidental
IDs in prose do not count. See the [mapping guide](../../../docs/TICKET_MAPPING.md) for setup,
provider IDs, examples, and `devtrack ticket link` corrections.

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
