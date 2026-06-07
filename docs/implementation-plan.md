# DevTrack — Proposed Architecture Implementation Plan

**Version:** 3.x → 4.0  
**Based on:** `docs/devtrack-architecture.html` § 11–13  
**Goal:** Evolve the current single-process FastAPI monolith into an event-driven, horizontally scalable architecture without changing the Go client or any external API contract.

---

## Table of Contents

1. [Guiding Principles](#1-guiding-principles)
2. [Target Architecture Overview](#2-target-architecture-overview)
3. [Component Breakdown](#3-component-breakdown)
4. [Data Layer](#4-data-layer)
5. [Implementation Phases](#5-implementation-phases)
   - [Phase A — Zero-risk quick wins (now, ~3 days)](#phase-a--zero-risk-quick-wins-now-3-days)
   - [Phase B — NATS + trigger worker (Sprint 1, ~1 week)](#phase-b--nats--trigger-worker-sprint-1-1-week)
   - [Phase C — PostgreSQL migration (Sprint 2, ~1 week)](#phase-c--postgresql-migration-sprint-2-1-week)
   - [Phase D — Observability + admin SSE (Sprint 3, ~1 week)](#phase-d--observability--admin-sse-sprint-3-1-week)
   - [Phase E — Multi-tenancy (Future)](#phase-e--multi-tenancy-future)
   - [Phase F — Kubernetes packaging (Future)](#phase-f--kubernetes-packaging-future)
6. [Dependency Map](#6-dependency-map)
7. [Directory Structure After Refactor](#7-directory-structure-after-refactor)
8. [Migration Risks & Mitigations](#8-migration-risks--mitigations)
9. [Definition of Done per Phase](#9-definition-of-done-per-phase)

---

## 1. Guiding Principles

- **The Go client does not change.** The HTTP/JSON boundary defined in `docs/HTTP_API.md` is frozen. Internal server decomposition is invisible to the client.
- **Evolution, not rewrite.** Every phase produces a deployable, working system. No "big bang" cutover.
- **Graceful degradation is preserved.** Optional subsystems (NLP, RAG, bots) remain independently toggle-able via env vars. Missing dependencies never crash the process.
- **No new external runtime unless justified.** NATS (~20MB binary) is the only new infrastructure dependency before PostgreSQL. Redis is already assumed present for session management.
- **Secrets stay in env vars.** No new secret-store model is introduced. The existing `.env` / vault-injection pattern extends cleanly.

---

## 2. Target Architecture Overview

```
devtrack_client (Go)
        │
        │  HTTPS POST /trigger/*
        ▼
┌─────────────────────────────────────────────────────────────┐
│  API Gateway  (thin Go process OR thin FastAPI router)       │
│  • Validate X-DevTrack-API-Key  (<1ms)                      │
│  • Ack 200 OK                   (<5ms total)                │
│  • Publish event to NATS JetStream                          │
└──────────────────────┬──────────────────────────────────────┘
                       │
                  NATS JetStream
                  (subjects: trigger.commit, trigger.timer,
                   trigger.ticket_sync, webhook.azure,
                   webhook.github, …)
                       │
         ┌─────────────┼───────────────┐
         ▼             ▼               ▼
  Trigger Worker   Webhook Handler   Admin Server
  (Python process) (Python process)  (FastAPI + HTMX)
  NLP · LLM        Route events      SSE live stats
  PM Sync          Notify            JWT auth
  Reports          Dedup             Audit log
         │             │               │
         └─────────────┴───────────────┘
                       │
             ┌─────────┴──────────┐
             ▼                    ▼
        PostgreSQL             Redis
        + pgvector             sessions
        (primary DB)           PM API cache
        embeddings             health TTLs

   Background (standalone processes, read from PG):
   • alert_poller
   • telegram_bot
   • slack_bot
```

---

## 3. Component Breakdown

### 3.1 API Gateway

| Attribute | Detail |
|---|---|
| **Language** | Go (thin wrapper reusing `devtrack_client` transport logic) or a minimal FastAPI router with no business logic |
| **Responsibility** | API key validation only; immediate 200 ack; NATS publish |
| **Does NOT do** | NLP, LLM, PM calls, database writes, session management |
| **NATS subject scheme** | `devtrack.trigger.<type>` (e.g. `devtrack.trigger.commit`), `devtrack.webhook.<source>` |
| **Scaling** | Stateless; run as many as needed behind a load balancer |
| **Auth** | `X-DevTrack-API-Key` header — same key, same validation logic, just moved earlier in the stack |

### 3.2 NATS JetStream

| Attribute | Detail |
|---|---|
| **Role** | Durable message broker between gateway and workers |
| **Why not Kafka** | Kafka requires JVM + ZooKeeper/KRaft (512MB+ baseline); NATS is a single ~20MB Go binary with built-in persistence |
| **Why not RabbitMQ** | Adds a new runtime (Erlang); NATS is closer to the existing Go stack |
| **Delivery guarantee** | At-least-once via JetStream durable consumers |
| **Horizontal scaling** | NATS queue groups — `docker compose up --scale trigger-worker=N` distributes load automatically, no sticky routing |
| **Streams to create** | `TRIGGERS` (subjects: `devtrack.trigger.*`) and `WEBHOOKS` (subjects: `devtrack.webhook.*`) |
| **Retention policy** | Work queue (delete on ack); max age 24h as dead-letter window |
| **Config surface** | `NATS_URL`, `NATS_STREAM_NAME`, `NATS_CONSUMER_GROUP` env vars |

### 3.3 Trigger Worker

| Attribute | Detail |
|---|---|
| **Language** | Python 3.12 |
| **Entry point** | `devtrack_server/workers/trigger_worker.py` |
| **Subscribes to** | `devtrack.trigger.*` queue group |
| **Pipeline** | Context enrichment → LLM-only NLP → LLM enhancer → task matcher → PM sync → report generation |
| **CPU-bound work** | None after spaCy removal — all I/O-bound, safe inside asyncio event loop |
| **Dependencies** | `nats-py`, existing `llm/`, `azure/`, `github/`, `jira/` clients |
| **Scaling axis** | Message throughput — add more workers via `--scale` |
| **State** | Stateless between messages; all state in PostgreSQL |

### 3.4 Webhook Handler

| Attribute | Detail |
|---|---|
| **Language** | Python 3.12 |
| **Entry point** | `devtrack_server/workers/webhook_worker.py` |
| **Subscribes to** | `devtrack.webhook.*` queue group |
| **Responsibility** | Route inbound Azure/GitHub/GitLab/Jira events; dedup by event ID; write to PostgreSQL; trigger OS/Telegram/Slack notifications |
| **HMAC validation** | Moved into worker (was in gateway); event rejected and nacked if signature invalid |
| **State** | Event dedup table in PostgreSQL |

### 3.5 Admin Server

| Attribute | Detail |
|---|---|
| **Language** | Python 3.12, FastAPI |
| **Entry point** | `devtrack_server/admin/server.py` (currently `admin/` within webhook_server.py) |
| **Routing** | Extracted from `webhook_server.py` into its own mountable FastAPI app |
| **Reads from** | PostgreSQL (read-heavy; separate read replica optional) |
| **Live stats** | Replace HTMX 30s polling with Server-Sent Events (`GET /admin/_stream/stats`) |
| **Auth** | JWT cookie (unchanged); scrypt password hashing (unchanged) |
| **Port** | Configurable via `ADMIN_PORT`; or `ADMIN_EMBED=true` mounts on port 8089 |

### 3.6 Background Processes

All are standalone Python processes that read/write PostgreSQL directly. They do not communicate via NATS.

| Process | Current location | Notes |
|---|---|---|
| `alert_poller` | `backend/alert_poller.py` | No change to logic; swap SQLite reads for asyncpg |
| `telegram_bot` | `backend/telegram/` | No change to logic |
| `slack_bot` | `backend/slack/` | No change to logic |

---

## 4. Data Layer

### 4.1 PostgreSQL (primary store)

**Replaces:** SQLite (`Data/db/devtrack.db`)

| Concern | Approach |
|---|---|
| **Client library** | `asyncpg` in trigger hot path; `SQLAlchemy 2.x` + `alembic` for schema migrations and admin queries |
| **Schema migration tool** | Alembic — migration files in `devtrack_server/db/migrations/` |
| **Connection pool** | `asyncpg.create_pool(min_size=2, max_size=10)` per worker process |
| **Vector search** | `pgvector` extension — replaces ChromaDB for RAG embeddings |
| **Tables to migrate** | `triggers`, `task_updates`, `health_snapshots`, `notifications`, `alert_state`, `users`, `api_keys`, `audit_log`, `learning_state`, `communication_samples`, `user_profiles` |
| **Multi-writer safety** | PostgreSQL handles concurrent writes from multiple trigger workers correctly; SQLite single-writer limitation gone |
| **Config vars** | `DATABASE_URL` (standard DSN); `DB_POOL_MIN`, `DB_POOL_MAX` |

### 4.2 Redis

**Role:** Hot cache layer; not a primary store.

| Use case | TTL |
|---|---|
| PM API response cache (ticket lists, user info) | 60–300s configurable |
| Session tokens (admin JWT) | `JWT_EXPIRY_SECS` |
| Health check snapshots | 30s |
| Rate limiting counters (API key) | 60s rolling window |
| NATS message dedup IDs | 24h |

**Config vars:** `REDIS_URL`, `REDIS_POOL_SIZE`

### 4.3 pgvector — replacing ChromaDB

| Attribute | Detail |
|---|---|
| **Extension** | `pgvector` installed on the same PostgreSQL instance |
| **Table** | `embedding_samples(id, user_email, context_type, text, embedding vector(768), created_at)` |
| **Model** | `nomic-embed-text` via Ollama (unchanged) |
| **Query** | `ORDER BY embedding <=> $1 LIMIT 5` (cosine similarity) |
| **Benefit** | Eliminates separate ChromaDB process and `~100MB` embedding store; unified backup/restore |

---

## 5. Implementation Phases

### Phase A — Zero-risk quick wins (now, ~3 days)

**Scope:** Changes to `devtrack_server/` only. No new infrastructure. No breaking changes.

#### A-1: Connection pooling in PM clients (~1 day)

**Problem:** Each PM API call creates a new `aiohttp.ClientSession`, incurring a full TLS handshake (50–200ms overhead) every time.

**Files to change:**
- `devtrack_server/backend/github/client.py`
- `devtrack_server/backend/azure/client.py`
- `devtrack_server/backend/jira/client.py` (and `gitlab/client.py` if applicable)

**Pattern:** Class-level `_session` singleton with lazy init and closed-session detection. `TCPConnector(limit=20, ttl_dns_cache=300)`.

**Validation:** Benchmark PM sync latency before/after with `time.perf_counter()` in the trigger handler. Target: 50–200ms reduction per call visible in `narrative.log`.

---

#### A-2: Replace spaCy with LLM-only NLP (~2 days)

**Problem:** spaCy adds 50MB model weight, a slow import, and CPU-bound parsing that blocks the asyncio event loop.

**Files to change:**
- `devtrack_server/backend/nlp_parser.py` — replace spaCy entity extraction with a structured LLM prompt returning JSON (`ticket_id`, `project`, `action_verb`, `status`, `time_spent`)
- `devtrack_server/backend/webhook_server.py` — remove `nlp_available` gate; LLM path becomes primary, not fallback
- Remove `spacy` from `devtrack_server/pyproject.toml` dependencies

**Prompt design:** A single system prompt with a JSON schema example and few-shot examples; use `json_mode=True` via the existing `llm/` abstraction.

**Test coverage:** Update `devtrack_server/backend/tests/test_nlp_parser.py` to mock the LLM provider and assert the same structured output fields.

**Validation:** Run existing tests; verify narrative log shows NLP stage durations drop from ~150ms (spaCy) to <50ms for cached LLM calls.

---

### Phase B — NATS + trigger worker (Sprint 1, ~1 week)

**Goal:** Decouple HTTP ack from processing. Trigger workers become a separate process.

#### B-1: Add NATS to docker-compose (~0.5 days)

**File:** `docker-compose.yml`

Add a `nats` service:
- Image: `nats:latest` (single binary, ~20MB)
- Port: `4222` (client), `8222` (monitoring HTTP)
- Args: `--jetstream --store_dir /data`
- Volume: `nats_data:/data`

New env vars: `NATS_URL=nats://localhost:4222`

---

#### B-2: Gateway — publish-only mode (~1 day)

**Decision:** Keep the gateway as a thin FastAPI app (not a new Go binary) to minimise the number of moving parts. The existing `webhook_server.py` becomes the gateway.

**Changes to `webhook_server.py`:**
- Remove all direct calls to NLP, LLM, PM sync from `/trigger/*` handlers
- Replace with: validate API key → publish JSON payload to NATS subject → return `{"status": "queued"}` with 200
- Keep `/health`, `/admin/*`, `/boardroom`, and `/webhooks/*` routes on this process
- Add `NATS_URL` config accessor to `backend/config.py`

**NATS subject routing:**
| HTTP endpoint | NATS subject |
|---|---|
| `POST /trigger/commit` | `devtrack.trigger.commit` |
| `POST /trigger/timer` | `devtrack.trigger.timer` |
| `POST /trigger/ticket_sync` | `devtrack.trigger.ticket_sync` |
| `POST /trigger/workspace_reload` | `devtrack.trigger.workspace_reload` |
| `POST /webhooks/azure` | `devtrack.webhook.azure` |
| `POST /webhooks/github` | `devtrack.webhook.github` |

---

#### B-3: Trigger worker process (~2.5 days)

**New file:** `devtrack_server/workers/trigger_worker.py`

**Structure:**
- Async NATS subscriber using `nats-py` (JetStream durable consumer, queue group `trigger-workers`)
- Subscribes to `devtrack.trigger.*`
- For each message: deserialise JSON → run existing pipeline (context enrichment → LLM NLP → task matcher → PM sync → report) → ack
- On unhandled exception: nack with no redelivery + log to narrative; do not crash process
- Graceful shutdown on `SIGTERM`: drain in-flight messages before exit

**Process management:** `devtrack_server/Procfile` or `docker-compose.yml` `trigger-worker` service. The Go daemon's subprocess management (`daemon.go`) continues spawning `webhook_server.py` (gateway); trigger workers are managed by docker compose or systemd separately.

**Scaling:** `docker compose up --scale trigger-worker=4` — NATS queue groups distribute messages automatically.

---

#### B-4: Webhook handler process (~1 day)

**New file:** `devtrack_server/workers/webhook_worker.py`

Same pattern as trigger worker but subscribes to `devtrack.webhook.*`. Extracts existing `WebhookEventHandler` logic from `webhook_handlers.py`. HMAC validation runs here before any processing.

---

### Phase C — PostgreSQL migration (Sprint 2, ~1 week)

**Goal:** Replace SQLite with PostgreSQL. Enable concurrent writes from multiple workers. Lay foundation for pgvector.

#### C-1: Schema design and Alembic setup (~1 day)

**New directory:** `devtrack_server/db/`
- `migrations/` — Alembic revision files
- `models.py` — SQLAlchemy 2.x declarative models (mirrors current SQLite schema + new indexes)
- `pool.py` — `asyncpg.create_pool()` factory, imported by workers

**Tables:** Mirror existing SQLite schema. Add:
- `tenant_id VARCHAR` column to all tables (nullable for now; used in Phase E)
- `embedding vector(768)` column on `communication_samples` (replaces ChromaDB)
- Indexes: `(user_email, context_type)` on embeddings; `(source, read, timestamp)` on notifications

---

#### C-2: Migration script (~1 day)

**New file:** `devtrack_server/scripts/migrate_sqlite_to_pg.py`

- Reads existing `Data/db/devtrack.db` with Python `sqlite3`
- Streams rows in batches of 500 into PostgreSQL via `asyncpg.executemany`
- Idempotent — safe to re-run (upsert on primary key)
- Produces a row-count reconciliation report on completion

---

#### C-3: Swap data access in workers (~2 days)

**Files to change:** Trigger worker, webhook worker, alert poller, admin server, all PM client modules that do direct DB reads.

**Pattern:** Replace all `sqlite3` / `aiosqlite` calls with `asyncpg` pool acquired from `db/pool.py`. The pool is initialised once at worker startup and shared across all coroutines in that process.

**SQLAlchemy** is used only in `admin/` routes where ORM-level queries (filtering, pagination) are more ergonomic than raw SQL.

---

#### C-4: pgvector replaces ChromaDB (~1 day)

**Files to change:** `devtrack_server/backend/rag/vector_store.py`

Swap ChromaDB collection calls for `asyncpg` queries using the `<=>` cosine distance operator. The embedding model (`nomic-embed-text` via Ollama) and the `embedder.py` module are unchanged. The `sample_indexer.py` API surface (`index_sample`, `retrieve_examples`) is unchanged — only the backing store swaps.

Remove `chromadb` from `pyproject.toml`. Keep `Data/learning/chroma/` directory around until migration is confirmed clean.

---

### Phase D — Observability + admin SSE (Sprint 3, ~1 week)

#### D-1: OpenTelemetry export from runtime-narrative (~3 days)

**Files to change:** `devtrack_server/backend/runtime_narrative/` (the narrative emitter)

**Approach:** Add an optional `OTelExporter` alongside the existing `JsonFileExporter`. When `OTEL_EXPORTER_OTLP_ENDPOINT` is set, export spans to the configured collector. The `StoryStarted` / `StageCompleted` / `StoryCompleted` events map directly to OTel span start/add-event/end.

**Libraries:** `opentelemetry-sdk`, `opentelemetry-exporter-otlp-proto-grpc` — both optional; gate on import.

**Output targets (operator's choice):** Grafana Tempo, Jaeger, DataDog, Honeycomb, AWS X-Ray (all accept OTLP).

**Config vars:** `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME=devtrack-server`, `OTEL_TRACES_SAMPLER` (default `always_on`; set `parentbased_traceidratio` for high-volume production).

---

#### D-2: Prometheus `/metrics` endpoint (~1 day)

**New file:** `devtrack_server/backend/metrics.py`

Expose counters and histograms via `prometheus-client`:
- `devtrack_triggers_total` (labels: `type`, `status`)
- `devtrack_trigger_duration_seconds` (histogram, labels: `type`)
- `devtrack_pm_sync_duration_seconds` (histogram, labels: `platform`)
- `devtrack_llm_request_duration_seconds` (histogram, labels: `provider`)
- `devtrack_nats_queue_depth` (gauge, polled from NATS monitoring API)

Route: `GET /metrics` — standard Prometheus scrape endpoint.

---

#### D-3: Admin console SSE live stats (~1 day)

**Files to change:** `devtrack_server/admin/server.py`, `devtrack_server/admin/templates/dashboard.html`

**Replace:** HTMX `hx-trigger="every 30s"` polling on the stats panel  
**With:** `GET /admin/_stream/stats` — standard `text/event-stream` SSE endpoint. Server pushes a stats JSON event whenever trigger counts or process health state changes (driven by PostgreSQL `NOTIFY` channel or a 5s background task).

Browser-native SSE; no new JS library. HTMX `hx-ext="sse"` handles the connection client-side.

---

### Phase E — Multi-tenancy (Future)

**Prerequisite:** Phase C complete (PostgreSQL with `tenant_id` columns).

**Scope:**
- `workspaces.yaml` gains a `tenant_id` field per workspace entry
- `WorkspaceRouter` propagates `tenant_id` to all DB writes
- Per-tenant LLM provider config: `LLM_PROVIDER_<TENANT_ID>` env var override
- Per-tenant rate limiting in Redis: `ratelimit:{tenant_id}:{api_key}` key
- Admin UI: tenant selector in user management

**What does NOT change:** HTTP API contract, Go client, workspace file format (additive-only `tenant_id` field with a sensible default).

---

### Phase F — Kubernetes packaging (Future)

**Prerequisite:** Phase B complete (decomposed workers).

**Deliverables:**
- `k8s/` directory with raw manifests (Deployment, Service, ConfigMap, HPA) for each component: gateway, trigger-worker, webhook-worker, admin-server, alert-poller
- `helm/devtrack/` chart wrapping the above with values for replica counts, resource limits, image tags, and secret references
- HPA for `trigger-worker` based on NATS consumer lag metric (KEDA `NatsJetStreamScaler`)
- `PodDisruptionBudget` for trigger workers (min 1 available during rolling updates)

**This is a packaging step, not a code change.** The architecture decomposition in Phase B is what makes this possible.

---

## 6. Dependency Map

Phases must be delivered in this order due to hard prerequisites:

```
A-1 (connection pooling)    ──── independent, ship immediately
A-2 (LLM-only NLP)         ──── independent, ship immediately

B-1 (NATS in compose)      ──── prerequisite for B-2, B-3, B-4
B-2 (gateway publish)      ──── requires B-1
B-3 (trigger worker)       ──── requires B-1, B-2
B-4 (webhook worker)       ──── requires B-1, B-2

C-1 (schema + alembic)     ──── prerequisite for C-2, C-3, C-4
C-2 (migration script)     ──── requires C-1
C-3 (swap data access)     ──── requires C-1, C-2
C-4 (pgvector)             ──── requires C-3

D-1 (OTel export)          ──── independent, can run in parallel with C
D-2 (Prometheus /metrics)  ──── independent
D-3 (admin SSE)            ──── requires C-3 (needs PG NOTIFY) or can stub with polling

E (multi-tenancy)          ──── requires C complete
F (Kubernetes)             ──── requires B complete
```

---

## 7. Directory Structure After Refactor

```
devtrack_server/
├── backend/
│   ├── config.py                   # adds NATS_URL, DATABASE_URL, REDIS_URL, OTEL_* vars
│   ├── webhook_server.py           # gateway only — validate, ack, publish to NATS
│   ├── webhook_handlers.py         # kept for backward compat; logic moves to webhook_worker
│   ├── llm/                        # unchanged
│   ├── azure/, github/, gitlab/,   # PM clients — connection pooling added (Phase A)
│   │   jira/
│   ├── admin/                      # extracted admin app (Phase B/D)
│   │   ├── server.py
│   │   └── templates/
│   ├── rag/
│   │   ├── embedder.py             # unchanged
│   │   ├── vector_store.py         # pgvector backend (Phase C-4)
│   │   └── sample_indexer.py       # unchanged API surface
│   ├── runtime_narrative/          # adds OTelExporter (Phase D-1)
│   └── metrics.py                  # new — Prometheus counters (Phase D-2)
│
├── workers/                        # new in Phase B
│   ├── trigger_worker.py
│   └── webhook_worker.py
│
├── db/                             # new in Phase C
│   ├── pool.py
│   ├── models.py
│   └── migrations/
│       └── versions/
│
├── scripts/
│   └── migrate_sqlite_to_pg.py     # new in Phase C
│
├── tests/
│   └── ...                         # existing + new worker tests
│
├── docker-compose.yml              # adds nats, postgres (if not already), redis
├── Procfile                        # trigger-worker, webhook-worker, gateway, admin
└── pyproject.toml                  # removes spacy, adds nats-py, asyncpg, pgvector, opentelemetry-*
```

---

## 8. Migration Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| NATS message loss during worker restart | Low | Medium | JetStream durable consumers with 24h retention window; redelivery on worker reconnect |
| PostgreSQL migration drops rows | Low | High | Migration script is idempotent; row-count reconciliation report before cutover; keep SQLite read-only for 2 weeks after cutover |
| spaCy removal regresses NLP quality | Medium | Medium | A/B test LLM-only output against spaCy output on 50 real commits before removing; fallback flag `NLP_BACKEND=spacy` retained for 1 sprint |
| pgvector cosine results differ from ChromaDB | Low | Low | RAG is advisory (style injection); worst case is slightly less personalized output, not a correctness failure |
| Connection pool exhaustion under burst load | Low | Medium | Set `DB_POOL_MAX=10` per worker, `REDIS_POOL_SIZE=10`; monitor with Prometheus `devtrack_db_pool_active` gauge |
| Workers fail to drain on SIGTERM | Low | Medium | `nats-py` drain() called in signal handler with 10s timeout; in-flight messages are re-queued by NATS on disconnect |
| Admin SSE connection leak | Low | Low | SSE handler tracks open connections in a set; removed on disconnect event |

---

## 9. Definition of Done per Phase

### Phase A
- [ ] PM sync latency in `narrative.log` reduced by ≥50ms per call (measured with `time.perf_counter`)
- [ ] All existing tests pass
- [ ] `spacy` removed from `pyproject.toml`; `uv sync` produces no spaCy-related install
- [ ] NLP output fields (`ticket_id`, `project`, `action_verb`, `status`, `time_spent`) match current schema in 100% of test cases

### Phase B
- [ ] `POST /trigger/commit` returns 200 in <5ms (p99 over 100 requests)
- [ ] Trigger worker consumes and processes messages from NATS; PM sync completes asynchronously
- [ ] `docker compose up --scale trigger-worker=2` successfully distributes messages across both workers (verify via NATS monitoring at `:8222`)
- [ ] Worker crash does not lose messages; redelivered on restart

### Phase C
- [ ] Migration script produces row-count reconciliation showing 0 discrepancies
- [ ] All trigger worker, webhook worker, and admin reads/writes use PostgreSQL
- [ ] pgvector cosine similarity results are within 5% of ChromaDB results on the same query set
- [ ] No `sqlite3` or `aiosqlite` imports remain in worker code paths
- [ ] `chromadb` removed from `pyproject.toml`

### Phase D
- [ ] Spans visible in Grafana Tempo (or Jaeger) when `OTEL_EXPORTER_OTLP_ENDPOINT` is set
- [ ] `GET /metrics` returns valid Prometheus text format; all counters increment on trigger processing
- [ ] Admin dashboard stats panel updates via SSE within 5s of a trigger arriving; no 30s polling requests in browser network tab

### Phase E
- [ ] Two workspaces with different `tenant_id` values route to different PM platforms without cross-contamination
- [ ] Per-tenant LLM override (`LLM_PROVIDER_TENANT_X=openai`) routes that tenant's triggers to the correct provider

### Phase F
- [ ] `helm install devtrack ./helm/devtrack` deploys all components to a local Kubernetes cluster (e.g. kind or minikube)
- [ ] HPA scales trigger workers from 1 to 4 under simulated load (100 triggers/minute)
