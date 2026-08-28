# DevTrack Git Commit Workflow

`devtrack git commit` wraps a normal Git commit with repository-aware message generation. Git still owns the commit; DevTrack must never make the developer's normal Git workflow depend on its AI or server.

## Interactive commit

Stage changes and provide an initial message:

```bash
git add path/to/file
devtrack git commit -m "describe the change"
```

DevTrack shows the proposed message and offers:

- `A` — accept and commit.
- `E` — enhance the current proposal.
- `R` — regenerate from the original message and staged diff.
- `Q` — queue the staged diff and message for later review when that hook is available.
- `C` — cancel without committing.

Enhance and regenerate share a maximum of five attempts. Invalid input does not consume an attempt. If standard input disappears, DevTrack accepts the current message rather than leaving the Git operation blocked.

## Preview only

```bash
devtrack git commit -m "describe the change" --dry-run
```

`--dry-run` (or `-n`) prints the generated message and exits without committing or opening the interactive review loop.

## After the commit

In an interactive terminal, DevTrack can ask for time spent and whether to push. Work that becomes a PM update or another outbound action must be staged through `pending_actions`; there is no direct-send fallback.

If the configured LLM or managed server is unavailable, DevTrack degrades gracefully. Inspect the repository with `git status` and use normal Git commands whenever needed.

## Related commands

```bash
devtrack commits list
devtrack commits enhance
devtrack commits review
devtrack sage ask "explain the staged changes"
devtrack sage do "prepare a conventional commit"
```

`devtrack commits` manages deferred commit work. `devtrack sage` is the separate Go-native repository assistant.
