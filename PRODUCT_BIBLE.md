# DevTrack Product Bible

> This document is the definitive specification of what DevTrack is, why it exists,
> and how it must be built. It does not change. Individual phases may be re-sequenced,
> features may be deferred — but the principles, the architecture, and the non-negotiables
> in this document are permanent. Any change to this document requires explicit,
> deliberate decision with written justification.

---

## The Problem

Developers spend 20–40 minutes every day on overhead that has nothing to do with writing
code: updating ticket states, writing standup notes, responding to PR review nitpicks,
logging time, producing EOD reports. This overhead is not just time — it is expensive
context switching. Every minute spent on a Jira ticket is a minute spent not coding,
and more importantly, a minute rebuilding the mental context that was interrupted.

Existing tools solve the wrong problem:
- **AI coding assistants** (Copilot, Claude Code, Cursor) help write code. They do not
  touch the overhead around it.
- **PM analytics dashboards** (LinearB, Swarmia, Gitential) give managers visibility.
  They do not reduce developer overhead.
- **Standup bots** (Geekbot, Range) ask you questions. They do not answer them for you.
- **PM platforms** (Jira, Azure DevOps, Linear) are the destination. They do not
  auto-populate themselves.

No tool absorbs the developer's meta-work. DevTrack does.

---

## The Vision

DevTrack is a silent background AI layer that absorbs all developer meta-work — ticket
updates, EOD reports, PR review cycles, time tracking — by watching what the developer
already does (committing code) and inferring everything else.

The developer's only obligation is to name branches sensibly. DevTrack handles the rest.

Over time, DevTrack gets better. Every correction trains it. Every approval signals
confidence. After months of use, the developer forgets it is running — but their PM
system is perfectly updated, their reports write themselves in their own voice, and their
PR review queues clear autonomously.

**The core promise:**

> You write code. DevTrack handles the rest — silently, accurately, in your voice,
> getting better every day.

---

## Non-Negotiables

These rules cannot be violated. They are not defaults. They are not configurable.
Any feature, refactor, or architectural decision that contradicts these rules is wrong
and must be reworked.

### 1. No prompts in the main flow
DevTrack never blocks the developer or asks for input as part of normal operation.
The daemon watches, infers, acts, and reports. Corrections happen through the feedback
interface — which is always optional, never blocking. If DevTrack cannot figure
something out, it logs it and moves on. It never waits for the developer.

### 2. Everything outbound is staged first
No action posts to any external system — PM tools, email, git — without first passing
through the pending actions queue. This applies even to high-confidence actions. The
queue is the trust primitive. It cannot be bypassed.

### 3. Confidence is always explicit
Every automated decision carries a confidence score. There is no silent overconfidence.
Low-confidence actions are prominently surfaced in the TUI and notifications.
High-confidence actions auto-approve faster. The confidence model is always visible
to the developer.

### 4. TUI and notifications are visibility only
The TUI, Telegram bot, and email notifications show what the daemon did and allow
corrections. They never gate features. The daemon runs identically whether any
interface is open or not. A feature exists in the daemon or it does not exist.
No feature is interface-specific.

### 5. All learning is local and private
Training data — git history, PR comments, meeting transcripts, user corrections — is
stored locally in SQLite and ChromaDB. Nothing leaves the machine for training purposes
without explicit opt-in (`TEAMS_ENABLED`, `TEAMS_TRANSCRIPTS_ENABLED`). Local LLM
(Ollama) is the default for all inference. Cloud LLM providers are optional upgrades.

### 6. Self-improvement is mandatory, not optional
Every correction the developer makes improves future accuracy for that specific pattern.
The learning loop cannot be disabled. DevTrack must measurably improve the longer it
is used. Stagnation at a fixed accuracy level is a product failure.

### 7. Everything DevTrack writes sounds like the developer
Every word DevTrack generates — commit messages, ticket comments, EOD reports, PR
responses — must be indistinguishable from the developer's own writing. Generic
AI-sounding output is a failure state. The voice layer is not a feature; it is a
requirement on every text output.

### 8. Never block on failure
If a ticket cannot be identified: log as unlinked, include in EOD report, move on.
If the PM API is down: queue the action for retry. If the LLM is unavailable: fall back
to templated output. If a PR review cannot be auto-resolved: escalate to the developer
with a clear explanation. The developer's workflow is never interrupted by DevTrack's
failures.

### 9. Offline-first
The core loop — git monitoring, ticket extraction, action queuing, EOD generation —
works with Ollama locally. No internet connection is required for daily operation.
Cloud services (OpenAI, Anthropic, MS Graph, PM APIs) are used when available and
gracefully skipped when not.

### 10. Multi-platform by default
Every capability works across Azure DevOps, GitHub, GitLab, and Jira. No feature is
designed for a single platform. Platform-specific shortcuts that create parity gaps
are forbidden.

### 11. Developer trust is earned progressively
Auto-approve thresholds start conservative (5-minute timeout requiring no rejection)
and tighten automatically as action accuracy improves. The developer can adjust thresholds
manually but DevTrack also adjusts them based on observed correction rates. Trust is
earned by track record, not assumed from day one.

### 12. The daemon is the product
The CLI, TUI, Telegram bot, and email are delivery channels for daemon output.
Features live in the daemon. Removing an interface removes visibility — it never
removes capability.

---

## The Developer Contract

This is the entirety of what a developer must learn to use DevTrack:

**1. Name your branches with the ticket ID:**
```
feature/PROJ-123-description
fix/ADO-456-null-check
hotfix/GH-789-regression
```

**2. Optionally prefix commits for higher confidence:**
```
PROJ-123: fix null check in auth flow
refs #456: update API response schema
```

**3. For explicit override:**
```bash
devtrack work start PROJ-123    # set active ticket manually
```

That is all. DevTrack reads signals in priority order:

| Signal | Example | Confidence |
|---|---|---|
| Branch name | `feature/PROJ-123-*` | High |
| Commit message prefix | `PROJ-123: description` | High |
| Git trailer | `refs: PROJ-123` | High |
| Active ticket (explicit) | `devtrack work start PROJ-123` | Explicit |
| Recent active tickets | last modified, assigned to me | Low — flagged |
| No signal found | — | Logged as unlinked, never blocked |

Ticket ID patterns are configurable per workspace (`ticket_pattern` in `workspaces.yaml`).
Default patterns cover the most common formats across all platforms.

---

## The Four Layers

### Layer 1 — Silent Background Engine
The daemon. Watches git via fsnotify, fires on cron schedule, extracts ticket context,
generates content via LLM, queues outbound actions. Never interrupts the developer.
Runs identically whether any interface is open or not.

**Responsibilities:**
- Git commit detection and ticket extraction
- Automatic ticket state transitions (branch created → In Progress; merged to main → Done)
- EOD pipeline: collect commits → group by ticket → generate narrative → queue report
- PR review event detection and classification
- Agentic review fix loop (see Layer 4 / Puppet Master)
- All LLM inference via Ollama (default) or configured cloud provider

### Layer 2 — Pending Actions Queue
The trust primitive. Every action the engine wants to take on the outside world — post
a comment, update a ticket state, send a report, push a fix — lands here first.

**Each queued action contains:**
- Action type and target (e.g., "post comment on PROJ-123")
- Generated content (the actual text to post)
- Confidence score
- Expiry time (auto-approves after this)
- Status: pending / approved / rejected / posted / failed

**Confidence-based timeouts:**
| Confidence | Default timeout | Meaning |
|---|---|---|
| >90% | 2 minutes | Act unless rejected |
| 70–90% | 5 minutes | Show prominently, act if not reviewed |
| <70% | 15 minutes | Flag, require explicit approval |
| New action type | 30 minutes | Always review the first time |

Timeouts tighten automatically as accuracy improves for that action type.

### Layer 3 — Feedback and Correction Interface
The TUI, Telegram bot, and email notifications. Shows what the engine did. Allows
corrections. Corrections become training data immediately.

**What the interface shows:**
- Live activity feed: what DevTrack just did
- Pending queue: actions awaiting approval, with confidence indicators
- Low-confidence flags: items that need a glance
- Today's summary: commits, tickets touched, actions taken
- System health: daemon status, LLM availability, PM connectivity

**What the interface allows:**
- Approve or reject pending actions
- Edit generated content before it posts
- Correct ticket mapping ("this commit was for PROJ-456, not PROJ-123")
- Override confidence thresholds
- View full action history

**What the interface does NOT do:**
- Ask "what are you working on?"
- Block the developer from coding
- Host any feature that the daemon does not also run headlessly

**Channel parity for corrections (immutable rule):**
Every correction capability — approve, reject, edit — must be available on at least
one non-TUI channel (Telegram, email, or CLI). A user who never opens the TUI must be
able to fully supervise and correct DevTrack. The TUI is a window, never a gate; if a
correction is only possible in the TUI, the TUI has become de facto mandatory and
non-negotiable #4 is violated.

### Layer 4 — Learning System
The engine that makes DevTrack improve. The learning system follows the **dialectic
user modeling pattern** — every interaction, not just explicit corrections, triggers
active reasoning that updates the developer's inferred model. The system grows more
accurate the longer it runs. This is non-negotiable: stagnation at a fixed accuracy
level is a product failure.

**Dialectic modeling — how it works:**
Every interaction (commit, approval, rejection, edit, correction) is a signal. After
each signal, a local reasoning pass runs via the persona model (see Voice Layer) and
asks: "What does this interaction reveal about how this developer thinks, writes, and
works?" The answer updates the stored user model. At generation time, the model is
queried to produce curated context — not raw memories, but reasoned inferences —
which inject into the generation prompt.

This is the pattern, not a dependency on any specific tool or service. The reasoning
engine is local (Hermes 3 via Ollama). The store is SQLite FTS5 (already in DevTrack).
Nothing leaves the machine.

**What every interaction type signals:**
- **Commit approved without edit** → voice model confirmed for this context type
- **Commit edited before posting** → diff between generated and final is high-signal training data
- **Ticket mapping corrected** → branch/commit pattern for this workspace updated
- **Action rejected outright** → strong negative signal; confidence for this type drops
- **Auto-approve timeout passed untouched** → implicit approval; mild positive signal
- **Report section deleted** → developer does not want this in reports
- **Report section kept verbatim** → structure confirmed; replicate in future

**What the user model contains:**
- Inferred writing patterns per context (commit, ticket comment, EOD report, PR response)
- Ticket mapping rules specific to this developer's naming style
- Autonomously generated skills — recurring patterns the system observed and codified
  without being told. Example: "This developer always opens ticket comments with the
  ticket number in brackets." Not programmed; emerged from evidence.
- Confidence history per action type per workspace
- Auto-approve threshold per action type (tightens as accuracy improves)

**The evidence hierarchy (immutable rule):**
The user model is derived from behavioral evidence — what the developer actually does.
It is never overridden by declarations of intent. If the developer edits `profile.md`
to say "I write formally" but 50 actual examples show informal writing, the evidence
wins. The profile is an explanation of what was observed, not a declaration of identity.
Contradictions between profile and evidence are flagged, not silently blended.

**Storage:**
- SQLite FTS5 — interaction log, inferred skills, confidence history, reasoning summaries
- ChromaDB — actual text examples for few-shot retrieval at generation time
- All local. All private. Portable: copy `Data/learning/` to a new machine.

---

## The Voice Layer

Every piece of text DevTrack generates must sound like the developer wrote it.
This is not aspirational. It is a hard requirement on every text output. A colleague
reading a ticket comment DevTrack posted must not be able to tell it was AI-generated.

### The persona model

DevTrack uses a strong persona-consistency model for all text generation. The pattern
is inspired by Nous Hermes models, which are specifically trained to treat persona
instructions as hard constraints rather than soft suggestions — maintaining voice,
tone, and stylistic patterns even when content varies widely. The persona model runs
locally via Ollama.

**At generation time, three signals combine:**

```
1. Curated user context (from dialectic user model — reasoned inferences about this developer)
        +
2. Few-shot examples (from ChromaDB — actual text this developer wrote, most relevant to task)
        +
3. Task instruction (what to generate: commit message, ticket comment, EOD report, etc.)
        ↓
Persona model generates text constrained by all three
```

Signal 1 tells the model *how* this developer writes (rules, inferred from evidence).
Signal 2 *shows* the model examples of actual writing. Signal 3 is the task.
The persona model reconciles all three — it does not hallucinate a blend when they
contradict, it flags the contradiction and uses evidence (Signal 2) as the authority.

### Voice training sources (ordered by friction)

**Tier 0 — Automatic, zero action required:**
Git commit history from all watched repos. On first `devtrack start`, the last 6 months
of commits are mined, embedded, and stored in ChromaDB. The developer already wrote
these. Instant baseline with zero configuration. The dialectic model begins reasoning
over these immediately to produce the first inferred user profile.

**Tier 1 — Automatic, uses existing auth:**
PR descriptions, issue comments, and review responses from GitHub, GitLab, and Azure
DevOps via already-configured tokens. Richer written signal, no extra auth required.
Runs as a background sync, updates ChromaDB and the dialectic model continuously.

**Tier 2 — One command:**
```bash
devtrack voice add "example of how I write status updates"
```
Manual fine-tuning. Optional. Highest signal-to-noise because the developer explicitly
chose these examples. Injected directly into ChromaDB, tagged as high-weight.

**Tier 3 — Opt-in, requires MS Graph:**
Teams messages. `TEAMS_ENABLED=true`. Captures written communication patterns across
channels, DMs, and meeting chats. Significantly richer than git history for informal
register and team-specific vocabulary.

**Tier 4 — Opt-in, highest signal for tone:**
Meeting transcripts and recordings from Teams. `TEAMS_TRANSCRIPTS_ENABLED=true`.
Only the user's speaking turns are extracted using speaker-ID filtering from the
transcript. Captures verbal communication patterns: how the developer frames uncertainty,
their register in standups vs. all-hands, characteristic opening and closing phrases.
The dialectic model uses verbal patterns to calibrate formality selection at generation
time. Stored locally, never shared.

### Voice matching requirements
- Commit message tone must match the developer's historical commit style exactly
- Ticket comment formality must match how the developer writes in that specific platform
- EOD report narrative must read as if the developer summarised their own day
- PR fix commit messages must be indistinguishable from the developer's manual commits
- Meeting-derived tone signals inform casual vs. formal register selection per context
- Generic AI-sounding phrases ("Certainly!", "I've updated...", "This commit...") are
  failure signals — if generated text sounds like an AI, the voice layer is broken

### The profile — transparency without authority

The developer's inferred behavioral profile is stored as a human-readable markdown
file (`Data/learning/profile.md`). This file is written by DevTrack from evidence.
The developer can read it to understand what the system learned. They can correct
wrong observations. They cannot declare new identity traits that contradict evidence —
such entries will be flagged as unverified until supporting evidence accumulates.

The profile is a mirror, not a mask.

---

## The Puppet Master — PR Review Loop

When a PR receives review comments (from Copilot, Devin, a human reviewer, or any bot):

```
Review comment detected
        ↓
Classify: auto-fixable OR needs human
        ↓
Auto-fixable → spawn coding agent (Claude Code CLI / Copilot CLI)
        ↓
Agent applies fix → commits in developer's voice → pushes
        ↓
DevTrack polls review state
        ↓
New comments? → classify again → loop
        ↓
All resolved → notify developer: "PR #123 approved"
Stuck        → notify developer: "PR #123 needs you — [specific blocker with context]"
```

**Auto-fixable (DevTrack handles autonomously):**
- Formatting, whitespace, line length
- Naming conventions (variables, functions, files)
- Missing documentation or comments
- Linting violations
- Obvious simple logic corrections
- Import ordering, unused imports

**Needs human (DevTrack escalates immediately):**
- Architecture decisions
- Product or business logic questions
- Security implications
- Ambiguous intent where multiple valid solutions exist
- Anything the agent tried and failed twice

**Developer experience:**
Push code. Move to the next ticket. Receive one notification: approved or blocked with
exact context. The PR review cycle ran without touching you.

**Coding agent integration:**
DevTrack invokes Claude Code CLI or Copilot CLI as a subprocess, passing the review
comment as context. The agent choice is configurable (`REVIEW_AGENT=claude-code` or
`copilot-cli`). DevTrack manages the loop — it is not the agent; it orchestrates the
agent.

---

## Build Phases

Phases are sequenced in trust-arc order: safe → accurate → automated → autonomous.
Do not skip phases. Each phase is a usable, testable increment.

### Phase 0 — Foundation reset
Remove TUI prompts from the timer trigger and commit trigger flows. These become
fully silent. The daemon no longer asks anything during normal operation.
Existing PM sync, LLM pipeline, and git monitor remain untouched.
**Exit criterion:** Daemon runs for a full day with no prompts shown.

### Phase 1 — Pending actions queue
New SQLite table: `pending_actions`. Every outbound PM action staged here. Configurable
timeout with auto-approve. Confidence score on every action. TUI and Telegram show queue.
Nothing posts to external systems without clearing this table.
**Exit criterion:** Developer can run for a week, review the queue each evening, and
trust that nothing unexpected posted.

### Phase 2 — Opinionated ticket extractor
Branch regex → ticket ID. Commit message keyword parsing. Active ticket fallback.
Unmatched commits logged as unlinked — never blocked. Configurable `ticket_pattern`
per workspace in `workspaces.yaml`.
**Exit criterion:** >80% of commits correctly mapped to tickets without any developer
configuration beyond standard branch naming.

### Phase 3 — Silent commit handler
On every commit: extract ticket → draft comment in developer's voice → stage in pending
queue → auto-transition ticket state (To Do → In Progress). Replaces TUI commit prompt
entirely.
**Exit criterion:** Developer commits normally. Ticket is commented and state-transitioned
within the auto-approve window. Developer did nothing except commit.

### Phase 4 — EOD pipeline
Cron fires at configured time (`eod_time` per workspace). Query today's commits from
SQLite. Group by ticket. LLM generates per-ticket summary in developer's voice. All
actions staged in pending queue. Email and Telegram deliver the report.
**Exit criterion:** Developer receives an accurate EOD email every evening without
doing anything. Report reads like they wrote it.

### Phase 5 — Voice training, low friction
Auto-seed ChromaDB from git commit history on first start (Tier 0). Background sync
of PR and issue comments via existing PM tokens (Tier 1). `devtrack voice add` for
manual examples (Tier 2). Teams messages as opt-in Tier 3. Meeting transcripts as
opt-in Tier 4. `devtrack voice status` shows training coverage and confidence.
The dialectic reasoning model begins from Tier 0 — first start produces the first
inferred profile with zero developer action.
**Exit criterion:** After one week, generated text passes the "did I write this?"
test for the developer without any manual profile editing.

### Phase 6 — Dialectic self-improvement
Every interaction feeds the reasoning loop: Hermes 3 (Ollama) runs a local reasoning
pass after each commit, approval, rejection, and edit. Inferences stored in SQLite
FTS5. Autonomously generated skills emerge from recurring patterns. Correction
interface in TUI lets developer flag wrong inferences — these become high-weight
training signals. Confidence thresholds per action type adjust continuously.
Profile transparency: `devtrack voice status` shows what was inferred and from which
evidence. Developer can correct wrong inferences directly.
**Exit criterion:** After 30 days, correction rate on ticket mapping and generated
text is measurably lower than day 1. At least three autonomous skills have emerged
without developer input. Developer has extended at least one auto-approve threshold.

### Phase 7 — TUI as visibility and correction layer
Reframe TUI entirely: activity feed, pending queue with confidence indicators,
correction interface for low-confidence actions, today's stats, system health.
Remove all input prompts. The TUI is a read + correct interface, never an input interface.
**Exit criterion:** Developer can open TUI at any time and immediately understand
everything DevTrack did in the last 24 hours and everything it is about to do.

### Phase 8 — PR review loop (puppet master)
PR review event detection via existing alert poller. LLM classification of each
comment. Headless invocation of Claude Code CLI or Copilot CLI. Fix-commit-push loop.
Escalation to developer with full context when stuck. Developer notified only on
completion or genuine blocker.
**Exit criterion:** Developer pushes a PR with formatting and naming review comments,
moves to next ticket, receives "PR approved" notification without touching the PR again.

---

## What DevTrack Is Not

These are explicit anti-features. Building them violates the product's identity.

- **Not a coding assistant.** DevTrack does not write features, suggest implementations,
  or review code quality. That is Copilot and Claude Code's job. DevTrack handles
  the workflow around the code, not the code itself.

- **Not a project management tool.** DevTrack does not replace Jira, Azure DevOps,
  GitHub Issues, or GitLab. It posts to them. The developer's PM platform of choice
  remains the system of record.

- **Not a team analytics dashboard.** DevTrack serves the individual developer.
  It has no org-level views, no manager dashboards, no team comparisons.

- **Not an interactive chatbot.** DevTrack does not answer questions, respond to
  commands mid-workflow, or engage in conversation during the developer's work session.
  Commands exist for configuration and correction, not for conversation.

- **Not cloud-dependent.** DevTrack is not a SaaS product. It runs on the developer's
  machine. Cloud integrations (PM APIs, Teams, email) are outbound connections to
  external systems the developer already uses — they are not infrastructure DevTrack
  depends on.

- **Not a surveillance tool.** DevTrack's learning data belongs to the developer.
  It is stored locally. It is not transmitted to any central server. It cannot be
  accessed by employers, team leads, or any third party.

---

## The Success Test

After two weeks of running DevTrack on a real project, the developer should be able
to answer yes to all of these:

1. I spent zero deliberate time updating ticket states.
2. I received an EOD report every day that accurately described what I did.
3. The generated ticket comments and reports sound like I wrote them.
4. At least one PR review cycle resolved without me touching it.
5. I forgot DevTrack was running at some point during the two weeks.
6. The accuracy of ticket mapping improved noticeably from day 1 to day 14.
7. I feel comfortable extending the auto-approve timeout because I trust what it posts.

If any of these fail, a non-negotiable has been violated and the product needs
to be reworked, not patched.

---

## Version History

| Date | Change | Author |
|---|---|---|
| 2026-06-10 | Initial version | Shashank Raj + Claude |
| 2026-06-10 | Learning system: dialectic user modeling pattern (inspired by Honcho/Hermes Agent); voice layer: persona model pattern (inspired by Nous Hermes); profile as mirror not mask | Shashank Raj + Claude |
| 2026-06-10 | Layer 3: channel parity rule for corrections — approve/reject/edit must exist on at least one non-TUI channel. Justification: the TUI-optional principle (non-negotiables #4, #12) is only enforceable if corrections are never TUI-exclusive. | Shashank Raj + Claude |

