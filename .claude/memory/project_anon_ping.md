---
name: Anonymous Usage Ping
description: Lightweight anonymous install and active-user telemetry so the user count can be displayed publicly. Separate from the opt-in feature telemetry in backend/telemetry.py.
type: project
---

Designed April 1, 2026. Not yet implemented.

**Why:** The existing `backend/telemetry.py` requires login + explicit opt-in — it will never produce a meaningful user count. To display "X installs" publicly, DevTrack needs a separate anonymous ping that fires without any account.

**How to apply:** This is a Go-side addition (`devtrack-bin/ping.go`). It is intentionally separate from the Python telemetry system and must stay that way. Never merge these two systems.

---

## What Gets Sent (nothing more)

```json
{
  "id":          "550e8400-e29b-41d4-a716-446655440000",
  "fingerprint": "sha256:3a7bd3e2360a3d29eea436fcfb7e44c735d117c42d1c1835420b6b9942dd4f1b",
  "event":       "installed" | "active",
  "version":     "1.2.0",
  "os":          "darwin",
  "arch":        "arm64"
}
```

No IP stored server-side. No repo names, commit messages, usernames, or file paths.
- `id` = random UUID stored in `~/.devtrack/id`. Can rotate on reinstall.
- `fingerprint` = SHA-256 of stable hardware ID (macOS: `IOPlatformUUID`, Linux: `/etc/machine-id`, Windows: registry MachineGuid). Hashed before leaving the machine — server cannot reverse it. Stable across reinstalls and ID deletion.

## Uniqueness: Count by Fingerprint, Not ID

The server uses `fingerprint` as the primary deduplication key, not `id`.

- Same fingerprint seen again → update `last_seen`, do NOT increment install count
- New fingerprint → increment install count, store alongside `id`

This handles:
- Reinstall (new UUID, same fingerprint) → not double-counted
- User deletes `~/.devtrack/id` (new UUID, same fingerprint) → not double-counted
- Script generating fake UUIDs on one machine (same fingerprint) → counts once

## Server Rate Limiting (anti VM-farm)

Per IP, per hour: max 5 new fingerprints accepted. Excess returns 200 OK (don't reveal the limit) but is not counted. IPs are not stored permanently — only used for the rate-limit window.

---

## Client Side: `devtrack-bin/ping.go`

Three functions:
- `getOrCreateInstallID()` — reads `~/.devtrack/id`; creates random UUID + writes file (chmod 600) if missing
- `sendPing(event string)` — fires `POST DEVTRACK_PING_URL` in a goroutine (5s timeout, silent failure, never blocks)
- `shouldSendActivePing()` — reads `Data/last_active_ping` timestamp; returns true if > 24h ago

**Triggers:**
- `"installed"` — end of `devtrack init`, after ticket sync succeeds (fires once ever; guarded by a flag)
- `"active"` — on `devtrack start`, at most once per 24h

**Opt-out:** `devtrack telemetry off` creates `Data/telemetry_disabled`. Both this ping and `backend/telemetry.py` respect this file.

**Disclosure** (shown at end of `devtrack init`):
> DevTrack sends anonymous install and active-user pings using a hashed
> device identifier and a random ID. No personal data is collected.
> To disable: devtrack telemetry off

---

## Server Side: Cloudflare Worker

Temporary until the real SaaS backend is built. Deploy with `wrangler deploy` from `infra/ping-worker/`.

Endpoints:
- `POST /ping` — validate payload, write to Workers KV (`installs:<id>`, `active:<id>`)
- `GET /stats` — return `{ "installs": N, "active_30d": N }` by listing KV keys

KV namespaces: `INSTALLS` (no expiry, deduplicates by UUID) and `ACTIVE` (TTL 30 days for active count).

Free tier handles this scale indefinitely (100K requests/day, 1K writes/day).

---

## Config

Add to `.env_sample` as optional:
```
DEVTRACK_PING_URL=https://ping.devtrack.dev
```

The URL is hardcoded as default in `ping.go` so it works without `.env`. Empty string disables pings.

---

## README Badge (after worker is live)

```markdown
![Installs](https://img.shields.io/badge/dynamic/json?url=https://ping.devtrack.dev/stats&query=installs&label=installs&color=blue)
```

---

## Files to Create/Change

- `devtrack-bin/ping.go` — new file, all ping logic
- `devtrack-bin/cli.go` — call `sendPing("installed")` from init, `sendPing("active")` from start
- `infra/ping-worker/` — new directory, Cloudflare Worker JS + wrangler.toml
- `.env_sample` — add `DEVTRACK_PING_URL`
- `README.md` — add install count badge once worker is live
