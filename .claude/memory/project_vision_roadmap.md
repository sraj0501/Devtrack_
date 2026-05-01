---
name: DevTrack Vision and Roadmap Assessment
description: Gap analysis between current state and "AI replacement of me" vision — four layers with completion percentages
type: project
---

## The Vision

"OpenClaw for developers" — an AI that talks like me, acts like me, comments like me. Can manage all developer activities (reporting, git, PR, project management, communication) autonomously. Manageable from a phone or while on vacation. Essentially an AI stand-in.

**Why:** The real value of AI is handling routine developer overhead (status updates, ticket management, PR reviews, team communication) so the developer can focus on coding or be away entirely.

**How to apply:** Every feature should be evaluated against this north-star. Priority: bidirectional integrations > remote access > full autonomy.

## Three-Layer Assessment (as of March 19, 2026)

### Layer 1: "Automate my reporting and git work" — ~90% done

| Capability | Status |
|---|---|
| Git commit monitoring + NLP parsing | WORKING |
| Timer-triggered TUI work update prompts | WORKING |
| AI-enhanced daily/weekly reports + email | WORKING |
| Personalization (Talk Like You + RAG) | WORKING |
| git-sage autonomous git operations | WORKING |
| Merge conflict auto-resolution | WORKING |
| Bidirectional Azure DevOps sync | IMPLEMENTED (untested) |
| Task matching to Azure DevOps tickets | WIRED into python_bridge.py |
| Project Manager <-> Azure sync | IMPLEMENTED (untested) |

### Layer 2: "Manage projects from my phone" — ~60% done

| Capability | Status |
|---|---|
| HTTP API / webhook server | IMPLEMENTED — FastAPI, Azure/GitHub/Jira endpoints |
| Inbound webhook processing | IMPLEMENTED — Azure DevOps events parsed |
| Telegram bot (commands + notifications) | WORKING — list, view, create, /plan, notifications |
| GitLab via Telegram | WORKING — /gitlab, /gitlabissue, /gitlabcreate |
| PM Agent via Telegram (/plan) | WORKING — platform picker → LLM decompose → create |
| Workload-aware task assignment | PLANNED |
| Push notifications for approvals | PARTIAL — OS + terminal via WebhookNotifier |
| Slack bot | NOT BUILT |

### Layer 3: "AI replacement of me" — ~35% done

| Capability | Status |
|---|---|
| Personalization engine (profile + RAG) | WORKING |
| Auto-update tickets from commits | IMPLEMENTED — commits + timer updates sync to Azure |
| Autonomous message sending (Teams/Slack) | PARKED (needs Power Automate) |
| Auto-comment on PRs | NOT BUILT |
| Accept/triage incoming tasks | PARTIAL — webhook receiver detects assignments |
| Vacation mode (auto-approve routine, escalate complex) | NOT BUILT |

### Layer 4: "Managed Cloud Mode" — ~0% built (designed March 19, 2026)

| Capability | Status |
|---|---|
| Cloud API (FastAPI, event endpoints) | DESIGNED — not built |
| WebSocket IPC (replaces local TCP IPC) | DESIGNED — not built |
| Encrypted credential vault | DESIGNED — not built |
| Cloud-hosted Telegram bot (always-on) | DESIGNED — not built |
| Cloud NLP + LLM pipeline | DESIGNED — not built |
| TUI dashboard (Bubble Tea) | DESIGNED — not built |
| Team management + workload routing | DESIGNED — not built |
| Billing / subscription tiers | DESIGNED — not built |

See `memory/project_managed_mode.md` for full architecture.

## Integration Depth (March 19, 2026)

| Integration | Read | Write | Bidirectional | In Daemon |
|---|---|---|---|---|
| Azure DevOps | Full | Full (comment, create, transition) | YES | YES |
| GitLab | Full | Full (create, comment) | YES | YES |
| GitHub | PR read/summarize | NO | NO | NO |
| Jira | Read-only stub | NO | NO | NO |
| Teams | Read (learning) | NO (Power Automate parked) | NO | NO |
| Email | Read (inbox) | Send (reports) | Partial | YES |

## Priority Roadmap

### Now (bot_automation branch)
1. **Test pass** — Azure client, webhook server, sync, bridge wiring
2. **Commit + push** — bot_automation branch (wiki, landing, daemon, CLI)

### Near-term
3. **Managed Mode Phase 1** — Cloud API + WebSocket IPC + API key auth
4. **TUI Dashboard** — Bubble Tea: login, credential vault, activity log
5. **Workload-aware assignment** — PM Agent assigns tasks based on team member load
6. **launchd plist** — macOS auto-start on login

### Medium-term
7. **Ticket Alerter** — Poll + OS/terminal notify (see CLAUDE.md)
8. **Slack bot** — Remote control parity with Telegram
9. **Managed Mode Phase 2** — Team accounts, shared credential vault, billing

### Long-term
10. **Autonomous communication** — Power Automate / email
11. **Vacation mode** — Confidence-based auto-approve with escalation
12. **Multi-repo monitoring** — Per-repo config via workspaces.yaml

## Key Decisions Log

**March 17, 2026** — Teams messaging: `ChatMessage.Send` requires admin consent. Chosen approach: Power Automate webhook (parked). Fallback: email via `send_mail()`.

**March 19, 2026** — Wiki deployment: Option 1 chosen — wiki deployed independently on Netlify, fetches docs from GitHub raw URLs in prod (`../docs/` locally). No build step.

**March 19, 2026** — Managed mode dashboard: TUI (Bubble Tea) chosen over web dashboard. Rationale: stays in terminal, consistent with product identity, works over SSH. Only browser touchpoint is initial OAuth login and billing.

**March 19, 2026** — Managed mode credentials: client-side encryption before transmission. Cloud stores ciphertext only. Local proxy pattern considered for PM credentials (cloud sends instructions, local daemon executes) to preserve "credentials never leave machine" for PM tokens.
