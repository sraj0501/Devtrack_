# Show HN: DevTrack — a local daemon that gives coding agents memory of your actual work

> Hold for the next public release. Current public tag: v3.0.10. The MCP and Phase 9 onboarding
> details below describe unreleased `main`.

I have been building DevTrack for about 18 months because I was tired of translating Git activity
into ticket comments and standup notes by hand.

It is a local Go daemon that watches multiple repositories, keeps work context in SQLite, and stages
outbound actions in one review queue. An optional Python server handles heavier integrations and
generation. Ollama is the default; cloud model providers are opt-in.

The newer angle is MCP. A coding-agent session can ask DevTrack for the active ticket, today's
commits, voice context, and pending actions instead of reconstructing that context from scratch.
DevTrack is not trying to be another coding agent; it sits underneath them as ambient memory.

The trust model matters more to me than the generation. Ticket comments, state changes, and reports
are staged with visible confidence before anything is sent.

The installation path on `main` is deliberately progressive: Git monitoring and MCP can work before
Python or a model is ready, while the optional server bootstraps in the background and `doctor`
reports what is available or degraded. The repository includes a reproducible demo covering commit
detection, a staged action, EOD preview, and MCP context.

Important release caveat: v3.0.10 is still the newest binary release and predates MCP plus this Phase
9 onboarding work. Build `main` to inspect the preview, or wait for the next tag for release binaries.

Repository: https://github.com/sraj0501/Devtrack_

I would especially value feedback on two things: whether the staged-action queue is the right trust
boundary for several agents, and where the progressive install still feels dishonest or confusing.

---

Draft evidence: repository README and architecture; TASK-121 reproducible demo; TASK-118–120 delivery
record in `engineer_log.md`; latest-tag check recorded in the seven-day evidence window. No adoption,
install-count, or time-saved claim is made.
