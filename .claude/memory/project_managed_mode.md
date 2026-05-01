---
name: DevTrack Managed Cloud Mode Architecture
description: Architecture decisions and design for the managed cloud version of DevTrack — local Go daemon + cloud-hosted AI pipeline, TUI dashboard
type: project
---

## Concept

Two modes in the same binary, controlled by `.env`:

```
DEVTRACK_MODE=local      # current default — fully offline, no cloud
DEVTRACK_MODE=managed    # cloud-hosted AI pipeline, local daemon only
```

**Why:** Local mode preserves the "100% local, zero cloud" story. Managed mode unlocks always-on Telegram, public webhooks, better LLMs, and team features — without changing the local UX.

## What Stays Local (both modes)

- `devtrack` Go binary + CLI
- Git monitoring (fsnotify)
- Cron scheduler
- SQLite local cache
- `.env` config file

## What Moves to Cloud (managed mode only)

- Python NLP + LLM pipeline
- Telegram bot (always-on, even when laptop is off)
- Webhook receiver (public URL, no ngrok)
- Assignment pollers
- Personalization / RAG (ChromaDB, MongoDB)
- PM API integrations — **OR** local proxy pattern (see Credential Decision below)

## Protocol Change

**Local:** Go daemon → TCP JSON → python_bridge.py (localhost:35893)
**Managed:** Go daemon → HTTPS POST → cloud API + persistent WebSocket for responses

```
devtrack daemon  →  POST /event/commit
                    Authorization: Bearer <api_key>
                    { repo, branch, message, diff_summary }

cloud            →  WebSocket push  →  daemon
                    { action: "comment", work_item: 88, text: "..." }
```

When `DEVTRACK_MODE=managed`, daemon skips starting local Python processes entirely.

## Credential Decision (open question)

Two options for PM credentials (Azure PAT, GitLab token, etc.):

**Option A — Cloud-stored (simpler UX)**
- User pastes tokens into TUI once
- Encrypted client-side before transmission (cloud stores ciphertext only)
- Cloud decrypts at processing time and makes PM API calls
- Risk: credentials leave machine (even encrypted)

**Option B — Local proxy (privacy-preserving)**
- Credentials stay in local `.env` / keychain, never transmitted
- Cloud sends structured instructions: `{ action: "comment", item: 88, text: "..." }`
- Local Go daemon executes the PM API call using local credentials
- More complex, extra round-trip, but preserves "credentials never leave machine"

**Current lean:** Option B for PM tokens (strong privacy story), Option A for LLM keys (already cloud-bound).

## TUI Dashboard (replaces web dashboard)

Built with **Bubble Tea** (Charm ecosystem) — same Go binary, new `devtrack manage` command.

```bash
devtrack login          # OAuth (opens browser once) or API key
devtrack manage         # full-screen TUI
devtrack logout
```

**Panels:**
- Overview — connection status, last sync, recent activity
- Credentials — add/rotate/revoke PM tokens (secure prompt, client-side encrypt)
- Team — invite members, view workload distribution
- Activity — live log of cloud-processed events
- Settings — LLM provider, notification prefs, sync config

**Charm libraries to use:**
- `bubbletea` — TUI framework
- `lipgloss` — styling
- `bubbles` — tables, lists, spinners, inputs
- `huh` — forms (login, credential entry)

Lives in `devtrack-bin/tui/` alongside existing CLI.

## Cloud Stack

```
devtrack-api      FastAPI — event endpoints, WebSocket hub
devtrack-worker   async queue processor (NLP → LLM → PM actions)
devtrack-bot      Telegram bot (always-on)
PostgreSQL        users, API keys, config
MongoDB           RAG samples, learning data, activity log
Redis             event queue, WebSocket pub/sub
```

**Deployment target (launch):** Railway / Render / Fly.io — no K8s needed initially.

## What Managed Mode Unlocks vs Local

| Capability | Local | Managed |
|---|---|---|
| Works without Python installed | ✗ | ✓ |
| Telegram works when laptop is off | ✗ | ✓ |
| Public webhook URL (no ngrok) | ✗ | ✓ |
| Better LLMs (GPT-4, Claude) | optional | default |
| Shared team account | ✗ | ✓ |
| Ollama / full local AI | ✓ | ✗ |
| Zero data leaves machine | ✓ | ✗ |

## Business Model

- **Local mode** — free, open source, always
- **Managed solo** — subscription, single user, cloud AI + always-on bot
- **Managed team** — per-seat, adds team management + workload-aware assignment

## Foundation Already Laid (March 28, 2026)

The client-server split is partially implemented:
- `devtrack-bin/server_config.go` shipped — `DEVTRACK_SERVER_MODE=external` makes daemon skip Python subprocess
- `Dockerfile.server` added — Python backend can run as a standalone container
- Go binary is now Python-free in releases (~5MB, pure Go)
- `DEVTRACK_SERVER_URL` env var accepted (used when mode=external)

Remaining for Phase 1: Replace TCP IPC with HTTPS POST + WebSocket when connecting to a remote server.

## Build Phases

**Phase 1 — Foundation**
- Cloud API with `/event/commit` and `/event/timer` endpoints
- WebSocket connection from Go daemon
- API key auth (`devtrack login`)
- Cloud LLM pipeline (NLP → enhance → respond)

**Phase 2 — TUI Dashboard**
- `devtrack manage` Bubble Tea TUI
- Credential vault (client-side encryption)
- Activity log panel

**Phase 3 — Always-on Services**
- Cloud Telegram bot (persists across laptop sleep/restart)
- Public webhook receiver (Azure/GitLab inbound events)

**Phase 4 — Team**
- Multi-user accounts
- Workload-aware assignment
- Shared activity log
- Billing / subscription tiers
