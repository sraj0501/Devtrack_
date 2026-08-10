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

**Positioning note on the server:** the Python server is the brains (NLP, voice,
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
dependency chain: Python + uv, spaCy model, ChromaDB, and above all Ollama model
pulls (several GB, 10+ minutes). No setup polish makes that instant. The fix is to
**sequence the wow moments in dependency order** so value arrives before the brains
finish booting:

1. **Minute 0–2, no Python.** Go binary is self-sufficient for the first wow:
   daemon starts, git monitor watches, MCP server answers from SQLite.
   `devtrack mcp setup` → ask Claude Code "what am I working on" → answer.
2. **Background bootstrap.** `uv sync`, spaCy model, `ollama pull` kick off
   non-blocking during first run — the non-negotiable #1 posture ("never block,
   never prompt") applied to installation.
3. **Second wow when ready.** When the voice pipeline comes online, notify:
   "Voice profile built from N commits — try `devtrack work report`." The EOD
   preview in the user's own voice is the moment that converts; it must not gate
   the install, and it must not wait until 6pm.

### Workstreams (in order — a Show HN spike is one-shot; friction fixes land first)

**9a — Kill setup friction**
- `devtrack setup` writes a complete working `.env`: all 12 required vars get sane
  defaults *written into the generated file* (visible and editable — the "no silent
  fallbacks" rule survives in spirit). Hard-fail stays for secrets only.
- Extends the Managed Install epic; server bootstrap becomes part of the same flow.

**9b — Script the wow**
- First-run flow after setup: Tier 0 voice mining kicks off, then print
  "Profile built from N commits. Try: `devtrack work report`" and
  "Run `devtrack mcp setup` and ask Claude Code what you're working on."
  The MCP moment is the screenshot people share.
- **Background server bootstrap with visible degradation map:** `devtrack doctor`
  (or upgraded `devtrack status`) shows honestly what works now and what is coming:
  `git monitoring ✓ · MCP ✓ · ticket sync ✓ · voice generation — downloading model (4.1 GB, ~8 min)`.
  Every server failure mode degrades to something the Go client still does (the
  optional-import discipline already enforces this server-side).
- **LLM fast lane:** detect an existing Ollama install with a usable model and use
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
| TASK-117 | `devtrack setup` writes complete `.env` with visible defaults; hard-fail secrets only | 9a |
| TASK-118 | Background server bootstrap (uv sync / spaCy / ollama pull non-blocking) + progress in `status`/`doctor` | 9b |
| TASK-119 | First-run wow script: Tier 0 mining kickoff + guided `work report` / `mcp setup` prompts | 9b |
| TASK-120 | Ollama detection + BYO-cloud-key fast lane in setup | 9b |
| TASK-121 | Demo GIF (commit → staged action → EOD preview) + quickstart polish | 9c |
| TASK-122 | devtrack_wiki homepage rewrite + repo description/topics | 9c |
| TASK-123 | Registry submissions (MCP lists, plugin directory) | 9d |
| TASK-124 | Launch content: Show HN + dev.to + LinkedIn via post-generator | 9d |

> **Numbering:** this table originally proposed TASK-110–117, but the project board has since
> issued TASK-110/111 (wiki + docs reconciliation, shipped) and TASK-112–116 (the PostgreSQL
> backend epic, in progress — 13 of 15 modules ported as of 2026-07-31, PRs #231-236, #240-246;
> one additional module was dead code removed under TASK-133, leaving only `webhook_server.py`).
> `Data/agent_logs/project_board.md` is the authoritative ID ledger; the
> adoption gate was renumbered to TASK-117–124 to match it. The Postgres epic
> is now numbered through TASK-139; TASK-140 is the active CI repair and the
> next open ID is TASK-141.

> **TASK-109 (repo cleanup) is done** — it took the 9c README overhaul early: the three-layer
> message (hook / differentiator / trust) now leads the README, and telemetry was flipped to
> opt-in so the "nothing leaves your machine" trust claim is actually true. What remains of 9c is
> the demo GIF and the wiki homepage.

---

## Deferred (Phase 10+ — re-rank once outside users exist)

- Headless orchestration (global agent control via MCP)
- Voice/dialectic Tier 4 (local Hermes persona model)
- GitLab `IsPRApproved`

All three add capability to a product with one user; Phase 9 adds users to a product
that is already capable. Outside-user feedback will re-rank this queue anyway.
