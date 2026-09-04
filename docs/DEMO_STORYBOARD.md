# Credential-free demo storyboard

This storyboard records DevTrack's real CLI and daemon output for the launch demo. It covers the
product loop without a PM token, email address, approval, or external write:

1. a commit on a ticket-named branch is detected;
2. the daemon records the resolved ticket and stages a comment with explicit confidence;
3. `devtrack eod` generates an on-demand narrative and stages it;
4. the Go-native MCP self-test reads local context without Python.

The companion script creates a disposable repository with `pm_platform: none`, streams output from
an MCP-capable `devtrack` binary, and removes the temporary workspace entry when it exits. It never
calls `devtrack queue approve`, never supplies `--email`, and never inserts a canned queue row.

## Preconditions

- Use a build from upstream `main` that passes `devtrack mcp status`. The latest public release is
  **v3.1.1** and includes MCP plus Phase 9 onboarding, so it can run this storyboard. See the README
  for platform installation and source-build commands.
- Run `devtrack setup` once with that build and choose Managed mode.
- Start DevTrack with `devtrack start`.
- Wait until `devtrack doctor` reports the Python service ready. MCP itself does not need it, but the
  voice-aware commit action and generated EOD narrative do.
- Keep real PM integrations disabled for the recording. The script's disposable workspace is always
  registered with `--pm none` regardless of the other configured workspaces.
- Run from a terminal wide enough to show the queue columns without wrapping.

No GitHub, GitLab, Jira, Azure DevOps, email, OpenAI, or Anthropic credential is required. Managed
mode still needs its documented local PostgreSQL connection, and local generation needs Ollama.

## Record the demo

From the repository root:

```bash
./scripts/demo.sh --record
```

The script pauses at each scene so the recorder controls the pacing. It prints live output only; if
commit staging cannot be observed in the daemon log, it stops instead of manufacturing a successful
transcript.

### Scene 1 — immediate local memory

Capture `devtrack mcp status`, followed by `devtrack mcp test`. Keep the
`get_active_context` response on screen long enough to show that the Go-native MCP command reads
the client's local SQLite context and is available on demand.

### Scene 2 — silent commit detection

The script creates a disposable repository, registers it as a `none` PM workspace, switches to
`feature/DEMO-101-standup`, and makes a real commit. Capture the commit completing normally; DevTrack
must not prompt or block it.

### Scene 3 — staging and confidence

Immediately before committing, the script records the existing confidence-bearing staging lines as
a baseline. It then reads the real daemon log until it finds the new commit's hash and a staging line
that was absent from that baseline. Capture both lines. They are evidence from this run, not an old
action or an illustrative transcript.

Then capture:

```bash
devtrack queue list
```

This is the local SQLite queue view. Depending on where the action originated, the PostgreSQL-backed
server action may be evidenced in the daemon log rather than mirrored as a client row. Do not imply
that an absent local row means the server bypassed staging, and do not insert a row solely for the
recording.

The script waits up to 120 seconds for local-model staging by default. Slower machines can set a
larger positive value without editing the script:

```bash
DEMO_STAGE_TIMEOUT_SECS=180 ./scripts/demo.sh --record
```

### Scene 4 — standup preview, staged before delivery

Capture:

```bash
devtrack eod
```

The command prints the generated narrative and its real `Queued as action <id>` line when server
staging succeeds. The script supplies neither an email nor an approval, so there is no delivery step.

### Scene 5 — MCP sees the day's context

Capture a second:

```bash
devtrack mcp test
```

The response should now be allowed to differ from Scene 1: it reflects whatever the local SQLite
client actually observed. Do not edit IDs, counts, confidence, timestamps, or generated text in post.

## Recording rules

- Crop paths or usernames visually if privacy requires it; do not replace runtime values with more
  impressive ones.
- Do not show `queue approve`, `queue reject`, a PM dashboard, or an email delivery in this demo.
- If Ollama falls back to a template, keep the template or rerun after the model is ready; never paste
  in a generated narrative from another run.
- If a scene fails, run `devtrack doctor` and fix readiness before recording again. Failure output is
  not a successful demo scene.
- The existing GIFs under `devtrack_wiki/wiki/assets/` are historical assets, not proof of this run.
  Replace them only with a capture produced from this storyboard.

## Script preflight only

To check prerequisites and command availability without creating a repository or changing workspace
configuration:

```bash
./scripts/demo.sh --check
```
