# Next Steps — Repositioning & Phase 9 (Adoption Gate)

_Drafted 2026-07-02 from strategy discussion. Companion to `PRODUCT_BIBLE.md` — this
document does not change direction; it changes emphasis and sequencing. The Bible
defines the product; this defines how it earns users._

---

## Why now

DevTrack started 1.5 years ago, before AI harnesses matured. The harness explosion
(Claude Code, Cursor, Copilot agents) commoditized **one third** of the original value
proposition and made another third **more valuable**:

- **Commoditized:** on-demand generation. "AI writes your commit message / ticket
  comment" is table stakes — every harness does it when invoked.
- **Uncommoditized (and already built):**
  1. **Ambience.** Harnesses are session-based; they exist while invoked, then forget.
     Nothing in the harness world runs at 6pm, groups today's commits by ticket, and
     stages an EOD report unprompted. An always-on daemon is a different — and still
     empty — category.
  2. **Memory across tools.** "DevTrack is the memory Claude Code lacks" (Bible,
     Vision). Every new harness that ships makes a shared local context layer *more*
     valuable. The wave is a tailwind if DevTrack positions *under* the harnesses,
     not beside them.
  3. **The trust primitive.** The pending-actions queue (confidence scores, earned
     auto-approve, `stage_action` MCP tool) is ahead of the market. As developers run
     multiple agents that all want to write to Jira/GitHub/email, "one reviewable
     staging queue for every outbound action" becomes a real need that did not exist
     when DevTrack started.

**Conclusion:** the value proposition does not need inventing — the Bible already
contains the right one. What is misaligned is **emphasis** (memory/ambience buried,
generation foregrounded) and **the funnel** (setup cost that filters out everyone but
the author). Goal is adoption, not revenue.

---

## The repositioning

Invert the story's altitude. Lead with the outcome; use the agent-era angle as the
differentiator, not the headline. Three layers, in order, in every external artifact:

| Layer | Message | Where it lives |
|---|---|---|
| **Hook** | "Never write a standup again. You commit; tickets update, EOD reports send — silently, in your voice, entirely on your machine." | README hero, wiki homepage, GitHub repo description/topics |
| **Differentiator** | "And it's the memory your AI agents lack — one `devtrack mcp setup` and Claude Code knows your active ticket, today's work, and how you write." | README second fold, Show HN framing |
| **Trust** | "Everything outbound is staged first. Local Ollama by default. Nothing leaves your machine." | README third fold — answers the objection that stops installs |

The Bible does not change; it already contains all three. Only the *first sentence*
of each external artifact changes.

**Positioning note on the server:** the Python server is the brains (LLM enrichment, voice,
dialectic learning, boardroom) and is *why* the differentiator claim is true — thin
harness wrappers have no personalization pipeline sitting on months of local data.
But externally it must be **invisible brains**: a stranger never learns there are two
codebases. They install `devtrack`; the rest is implementation detail.

**Anti-goal:** do not chase harnesses into their territory (code quality, agentic
coding). "What DevTrack Is Not" holds. The moat is being orthogonal: ambient, local,
private, underneath all of them. Corollary: do **not** rewrite the server in Go for
distribution's sake — the Python ML ecosystem is where the brains' leverage comes
from. The managed-install pattern (Go orchestrates, Python serves) is the right trade.

---

## Phase 9 — Adoption Gate ("the first ten minutes")

> **Exit criterion:** a stranger on a fresh machine goes from README to
> (a) a staged action and an on-demand EOD preview, and
> (b) Claude Code answering "what am I working on?" via DevTrack,
> in under ten minutes, without reading anything but the quickstart —
> with the minute-two wow requiring **no Python at all**, and the brains coming
> online in the background without the user waiting on them.

This phase is almost entirely **packaging and narrative, not new capability**. The
raw material exists: `devtrack mcp setup/test` runs standalone on SQLite (no Python),
`devtrack work report` generates the EOD on demand, and the Managed Install epic
(TASK-103–108) already sparse-clones the server and runs `uv sync`.

### The friction math (why sequencing matters)

The true cost of bringing the brains online is not DevTrack's code — it is the
dependency chain: Python + uv, optional ChromaDB, and above all Ollama model
pulls (several GB, 10+ minutes). No setup polish makes that instant. The fix is to
**sequence the wow moments in dependency order** so value arrives before the brains
finish booting:

1. **Minute 0–2, no Python.** Go binary is self-sufficient for the first wow:
   daemon starts, git monitor watches, MCP server answers from SQLite.
   `devtrack mcp setup` → ask Claude Code "what am I working on" → answer.
2. **Background bootstrap.** `uv sync` and `ollama pull` kick off
   non-blocking during first run — the non-negotiable #1 posture ("never block,
   never prompt") applied to installation.
3. **Second wow when ready.** When the voice pipeline comes online, notify:
   "Voice profile built from N commits — try `devtrack work report`." The EOD
   preview in the user's own voice is the moment that converts; it must not gate
   the install, and it must not wait until 6pm.

### Workstreams (in order — a Show HN spike is one-shot; friction fixes land first)

**9a — Kill setup friction**
- **TASK-117 complete:** `devtrack setup` writes a complete working environment file with all 12
  runtime defaults visible and editable; invalid overrides still fail clearly.
- **TASK-142 complete (PR #253):** configured-LLM enrichment now owns structured parsing and degrades
  to raw/template data without the obsolete NLP/spaCy surface.

**9b — Script the wow**
- First-run flow after setup: Tier 0 voice mining kicks off, then print
  "Profile built from N commits. Try: `devtrack work report`" and
  "Run `devtrack mcp setup` and ask Claude Code what you're working on."
  The MCP moment is the screenshot people share.
- **TASK-118 complete (PR #254) — background server bootstrap with visible degradation map:** `devtrack doctor`
  (or upgraded `devtrack status`) shows honestly what works now and what is coming:
  `git monitoring ✓ · MCP ✓ · ticket sync ✓ · voice generation — downloading model (4.1 GB, ~8 min)`.
  Every server failure mode degrades to something the Go client still does (the
  optional-import discipline already enforces this server-side).
- **TASK-120 complete (PR #256) — LLM fast lane:** detect an existing Ollama install with a usable model and use
  it immediately (don't force a specific pull). If `ANTHROPIC_API_KEY` /
  `OPENAI_API_KEY` is present, offer it for instant generation while the local model
  downloads, then default back to Ollama once it lands. Offline-first is the steady
  state, not necessarily minute one — the Bible already treats cloud providers as
  optional upgrades and `provider_factory.py` already chains them.

**9c — Rewrite the shopfront**
- README overhaul: three-layer message, 30-second GIF
  (commit → staged ticket comment → EOD preview), ten-minute quickstart, and the
  Bible's Success Test verbatim as the public promise.
- devtrack_wiki homepage mirrors the same narrative.
- GitHub repo description + topics updated to match the hook.

**9d — Distribute where harness users already look** *(only after 9a–9c land)*
- Submit to MCP server registries and awesome-MCP lists.
- Claude Code plugin/skill ecosystem listing.
- Show HN framed as: *"a local daemon that gives coding agents memory of your
  actual work"* — the uncrowded angle, not "AI updates your Jira tickets."
- Point the post-generator agent at the new narrative for long-form dev.to
  ("I spent 18 months building the tool that writes my standups") and LinkedIn.
  The 1.5-year build history is itself the content — lived experience outperforms
  product pitches on every one of these channels.

### Success signal

Assume no telemetry signal. TASK-109 shipped anonymous install/active pings, but they are
**opt-in and off by default**, so adoption numbers will be absent or unrepresentative by
design — don't plan to measure the launch with them.

The metric is: **the first issue or PR from a stranger.** Stars are vanity; a bug report
from someone unknown means a real install survived setup.

---

## Proposed task breakdown (TASK-117+, for project-vision to formalize)

| # | Task | Workstream |
|---|---|---|
| TASK-117 | **DONE** — `devtrack setup` writes complete environment with visible defaults; invalid overrides fail clearly | 9a |
| TASK-118 | **DONE (PR #254)** — background server bootstrap (`uv sync` / local `ollama pull` non-blocking) + durable progress/retry in `status`/`doctor` | 9b |
| TASK-119 | **DONE (PR #255)** — automatic Tier 0 mining/profile build + guided `work report` / `mcp setup` prompts | 9b |
| TASK-120 | **DONE (PR #256)** — Ollama detection + BYO-cloud-key fast lane in setup | 9b |
| TASK-121 | **DONE** — reproducible commit → staged action → EOD/MCP demo + quickstart polish | 9c |
| TASK-122 | **DONE** — devtrack_wiki homepage rewrite + reviewable repo description/topics | 9c |
| TASK-123 | **DONE** — evidence-backed registry matrix and held submission copy | 9d |
| TASK-124 | **DONE** — evidence-backed Show HN, dev.to, and LinkedIn drafts; updated for v3.1.1 and held pending authenticated channel sessions | 9d |
| TASK-142 | **DONE (PR #253)** — configured-LLM task enrichment with strict validation and a non-blocking raw/template fallback | 9a prerequisite |
| TASK-148 | **DONE (`c1329f7`)** — MCP handshake/tool metadata hardening plus reproducible, validated per-platform MCPB release packaging | MCP distribution |
| TASK-149 | **DONE (PR #257)** — synchronize README, wiki, durable memory, registry evidence, and post-TASK-148 release gates | Documentation |
| TASK-150 | **RELEASE DONE; EXTERNAL FOLLOW-UP** — v3.1.1, five privacy-compliant native MCPBs, checksums, and official MCP Registry record published; third-party forms/posts need authenticated owner sessions | Release |

> **Numbering:** this table originally proposed TASK-110–117, but the project board has since
> issued TASK-110/111 (wiki + docs reconciliation, shipped) and TASK-112–116 (the PostgreSQL
> backend epic, complete through PR #251; 14 of 15 modules were ported and one dead module was
> removed under TASK-133, leaving no production raw-`sqlite3` imports).
> `Data/agent_logs/project_board.md` is the authoritative ID ledger; the
> adoption gate was renumbered to TASK-117–124 to match it. TASK-117 is complete. TASK-142 is now
> assigned to the parser cleanup. TASK-143 reconciled the Phase 9 status records; TASK-144 fixed
> live-demo queue and trigger reliability. TASK-145 synchronized the full documentation set,
> TASK-146 restored the comprehensive beginner wiki, and TASK-147 reconciled refreshed dependencies,
> Windows SQLite behavior, and the HTTP contract. TASK-148 added MCPB build readiness without
> publishing a release or registry entry. TASK-149 synchronized the documentation surfaces after
> that work. TASK-150 subsequently shipped v3.1.0, the privacy-compliance v3.1.1 patch, and the official registry record. The next unused
> ID is TASK-151.

> **TASK-109 (repo cleanup) is done** — it took the 9c README overhaul early: the three-layer
> message (hook / differentiator / trust) now leads the README, and telemetry was flipped to
> opt-in so no product telemetry leaves the machine unless the user enables it. Configured PM,
> email, remote-server, and cloud-LLM integrations still receive the data required for their jobs.
> TASK-124 completed the evidence-backed launch drafts; publishing remains intentionally gated on a
> public release that contains the Phase 9/MCP work. v3.1.1 clears the technical packaging gate; publishing
> now depends on authenticated owner sessions for each external channel.

---

## Deferred (Phase 10+ — re-rank once outside users exist)

- Headless orchestration (global agent control via MCP)
- Voice/dialectic Tier 4 (local Hermes persona model)
- GitLab `IsPRApproved`

All three add capability to a product with one user; Phase 9 adds users to a product
that is already capable. Outside-user feedback will re-rank this queue anyway.
