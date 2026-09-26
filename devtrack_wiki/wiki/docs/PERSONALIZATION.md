# AI Personalization ("Talk Like You")

DevTrack can build a local writing profile from evidence you control. Generators that explicitly
load that profile can use it as style context; the profile does not prove that every AI path is
personalized.

---

## How It Works

The working `devtrack voice` routes collect samples from:
- local Git commit messages (`voice seed`);
- explicit manual examples (`voice add`); and
- configured GitHub, GitLab, or Azure PR descriptions/comments (`voice sync`).

Teams and Outlook are intended opt-in sources, but their separate communication-learning adapter is
not currently end-to-end available.

Profile generation writes a local `profile.md` with observed writing characteristics. Consumers must
load it explicitly; it is not injected into every AI prompt automatically.

Learning data stays local by default. If you explicitly configure a cloud LLM, remote server, Teams,
Outlook, or another external source, the data required for that selected integration leaves the
machine under its configured consent and provider policy.

> **Current `dev` limitation:** automatic local Git-history seeding is implemented through the
> Managed onboarding worker. The HTTP adapter behind several communication-learning commands still
> uses an incomplete `LearningIntegration` adapter. Status, enable, sync, reset, cron, test, and
> revoke call absent method names; profile calls an existing method before initialization. Treat all
> of these CLI paths as unavailable until the adapter is repaired. The sections below describe the
> intended command contract, not a completed end-to-end path.

---

## Working voice commands

```bash
devtrack voice seed
devtrack voice add --context comment "Fixed the null check in auth flow"
devtrack voice sync
devtrack voice profile
devtrack voice status
```

These commands use the maintained `/voice/*` HTTP routes. They are distinct from the legacy
communication-learning command group below.

## Legacy communication-learning contract (currently unavailable)

### Intended enable flow

```bash
devtrack enable-learning
```

This is the intended consent flow, but the current server adapter method is absent. Do not run it as
an operational privacy control until the adapter is repaired.

To collect from the past N days (default is 30):

```bash
devtrack enable-learning 14    # collect last 14 days
```

### Intended communication sync and cron

After enabling, run a sync to collect samples:

```bash
devtrack learning-sync          # collect new messages since last run
devtrack learning-sync --full   # force full 30-day re-collection
```

Set up a daily cron to keep the profile current:

```bash
devtrack learning-setup-cron    # installs daily sync at LEARNING_CRON_SCHEDULE
devtrack learning-cron-status   # check cron status
devtrack learning-remove-cron   # remove cron entry
```

---

## Legacy profile/test commands (currently unavailable)

### View Your Profile

```bash
devtrack show-profile
```

This intended top-level command currently calls the legacy adapter before initialization. Use
`devtrack voice status` to inspect corpus/profile state and `devtrack voice profile` to generate the
maintained local profile instead.

### Test a Response

The intended response-preview command is currently unavailable:

```bash
devtrack test-response "I finished the login module and it's ready for review"
```

### Check Status

```bash
devtrack learning-status
```

Intended to show consent status, sample count, and last sync time. The current server adapter calls
an absent `get_status` method, so this command is unavailable until the adapter is repaired.

---

## Configuration

| Variable | Description |
|---|---|
| `LEARNING_CRON_SCHEDULE` | Cron expression for daily sync (e.g. `0 20 * * *` for 8pm daily) |
| `MONGODB_URI` | Referenced by the incomplete Microsoft Teams adapter. It is never required for the maintained local Git/manual/PM voice paths. |

Samples and profiles live locally under `DATA_DIR/learning/`. Git history alone is enough to seed
voice evidence, but profile generation is a Python-server capability and therefore uses the required
server PostgreSQL deployment. The Go client remains SQLite-only.

### RAG Enhancement (Optional)

For more accurate style matching, DevTrack can use vector search (ChromaDB + `nomic-embed-text`) to retrieve the most similar past responses:

```bash
ollama pull nomic-embed-text    # one-time setup
uv sync --extra ai              # installs ChromaDB (devtrack_server)
```

Once installed, RAG is automatic — no extra configuration needed.

---

## Legacy consent reset contract (currently unavailable)

### Revoke Consent

```bash
devtrack revoke-consent
```

Intended behavior: revoke consent and stop future communication-source collection while preserving
existing data. The adapter method is currently absent, so the command is not a reliable control.

### Full Reset

```bash
devtrack learning-reset
```

Intended behavior: wipe collected learning data and reset consent. The adapter method is currently
absent. For the current release, stop DevTrack, back up anything needed, and use normal uninstall
without `--keep-data`; direct SQL deletion is unsupported.

---

## Profile consumers and boundaries

The Python personalization/report paths can load a generated profile and RAG examples. Availability
depends on the specific route and configured provider; a profile is not automatically applied to
every generator.

| Feature | How your style is used |
|---|---|
| Python commit-message enrichment | Supplies commit-context style and related examples |
| Ticket comments and work descriptions | Supplies comment/description style and related examples |
| DevTrack Sage knowledge | Future consumer; deterministic Markdown knowledge is not complete yet |
| Daily report generation | Matches your preferred format |
| Python planning/task descriptions | Uses task-context style and related examples |

If no profile exists, these features fall back to standard AI output — no errors, just no personalization.

---

## Data Storage

| Store | Location | Contents |
|---|---|---|
| Local files | `DATA_DIR/learning/` | Samples (JSONL), profile (JSON), consent |
| MongoDB (optional) | `MONGODB_URI` | Referenced by the incomplete Teams adapter; not required for the working Git/PM voice paths. |
| Vector store | `DATA_DIR/learning/chroma/` | Embeddings for RAG (if AI tier installed) |
