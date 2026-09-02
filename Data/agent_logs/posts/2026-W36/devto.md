# I Spent 18 Months Building the Tool That Writes My Standups

> Draft only — do not publish until the Phase 9 work has a public release. The newest public tag is
> v3.0.10; commands marked “on main” below are newer and unreleased.

I did not start DevTrack because I wanted another developer dashboard. I started it because the same
work kept happening twice: once in Git, then again in a ticket, then a third time when somebody asked
for a standup update.

The first version was mostly generation. Give a model a commit and ask it to write something useful.
That was interesting in 2025. It is ordinary now. Coding agents can already write commit messages and
summaries when asked.

The part that stayed useful was the part that does not wait to be asked.

DevTrack is a local Go daemon. It watches work across repositories, records context in SQLite, and
stages outbound actions in a review queue. Its optional Python server adds ticket integrations,
reports, voice generation, and other heavier workflows. Ollama is the default model path; configured
cloud providers are optional.

The distinction sounds small, but it changes the job. A coding agent exists for a session. The daemon
is still there at the end of the day, when the useful question is not “can you summarize this diff?”
but “what did I actually finish across all the sessions and repositories I touched?”

## The feature I trust most is the one that stops the automation

Every outbound action is staged first. A proposed ticket comment, state transition, or report can be
inspected before it is sent. Confidence is visible. Auto-approval, where enabled, is earned rather
than assumed.

That queue became more important as the project gained integrations. Without one shared trust
boundary, each automation becomes another place that can quietly write the wrong thing.

## The installation problem was harder than the model problem

The Go-native path can monitor Git, retain local context, and expose MCP without Python or a model.
The richer server path needs Python, dependencies, PostgreSQL, and potentially a multi-gigabyte local
model. Pretending that chain is instant would make for a cleaner landing page and a worse product.

The unreleased Phase 9 work on `main` changes the sequence instead:

1. Deliver the Go-native value first.
2. Bootstrap the optional server in the background.
3. Show exactly which capabilities are ready, degraded, or still downloading.
4. Suggest the EOD and MCP demonstrations when their prerequisites are ready.

The repository now includes a reproducible five-scene demo, a ten-minute quickstart, background
bootstrap status, first-run voice seeding, and an LLM fast lane that can use an already-installed
Ollama model. Those are source-tree claims, not outside-user results. The next honest success signal
is still the first issue or pull request from somebody I do not know.

## What you can use today

The latest public release is v3.0.10. It contains the established Git and git-sage workflows and
significant Windows fixes. MCP setup and the Phase 9 onboarding path described above are newer work
on `main`; they are not in v3.0.10, and `devtrack upgrade` will not install them yet.

If you build from source, the quickstart in the repository labels that distinction directly. If you
prefer release binaries, wait for the next tagged release rather than assuming the preview commands
exist.

That is less exciting than saying “install it and everything is automatic.” It is also true.

DevTrack is here: https://github.com/sraj0501/Devtrack_

---

Draft evidence: `README.md` release warning and quickstart; `docs/NEXT_STEPS.md` Phase 9 sequencing;
`Data/agent_logs/engineer_log.md` seven-day window; TASK-121–123 and TASK-144 commits listed there;
`git tag --sort=-version:refname` showing v3.0.10 as the newest tag on 2026-09-02.
