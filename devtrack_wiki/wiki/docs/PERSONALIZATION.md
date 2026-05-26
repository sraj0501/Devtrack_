# AI Personalization ("Talk Like You")

DevTrack can learn your communication style and use it when generating commit messages, task descriptions, and work summaries — so the output sounds like you wrote it, not like a generic AI.

---

## How It Works

DevTrack collects samples of how you write from:
- Your Git commit messages
- Your Microsoft Teams chat history (with explicit consent)
- Your Outlook emails (with explicit consent)

It builds a local profile — formality level, average length, common phrases, sign-offs, emoji usage — and injects this style into every AI prompt.

All data stays on your machine. Nothing is sent to external services.

---

## Setup

### 1. Enable Learning

```bash
devtrack enable-learning
```

This starts an interactive consent flow that explains exactly what data will be collected, where it is stored, and how to revoke access. You must consent before any data is collected.

To collect from the past N days (default is 30):

```bash
devtrack enable-learning 14    # collect last 14 days
```

### 2. Sync Data

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

## Using Your Profile

### View Your Profile

```bash
devtrack show-profile
```

Shows the learned style: formality level, average response length, common phrases, detected tone, emoji preference, sign-off patterns.

### Test a Response

Generate a sample personalized response without affecting anything:

```bash
devtrack test-response "I finished the login module and it's ready for review"
```

### Check Status

```bash
devtrack learning-status
```

Shows consent status, sample count, and last sync time.

---

## Configuration

| Variable | Description |
|---|---|
| `LEARNING_CRON_SCHEDULE` | Cron expression for daily sync (e.g. `0 20 * * *` for 8pm daily) |
| `MONGODB_URI` | Optional — if set, samples are stored in MongoDB instead of local files |

If `MONGODB_URI` is not set, samples and profiles are stored locally under `DATA_DIR/learning/`.

### RAG Enhancement (Optional)

For more accurate style matching, DevTrack can use vector search (ChromaDB + `nomic-embed-text`) to retrieve the most similar past responses:

```bash
ollama pull nomic-embed-text    # one-time setup
uv sync --extra ai              # installs ChromaDB (devtrack_server)
```

Once installed, RAG is automatic — no extra configuration needed.

---

## Disabling / Resetting

### Revoke Consent

```bash
devtrack revoke-consent
```

Revokes consent and stops all future data collection. Existing data is preserved.

### Full Reset

```bash
devtrack learning-reset
```

Wipes all collected data (MongoDB + local files) and resets consent. Use this to start fresh.

---

## Integrations That Use Your Profile

Once a profile exists, it is automatically applied to:

| Feature | How your style is used |
|---|---|
| Commit message enhancement | Matches your length and formality |
| Work update descriptions | Writes in your voice |
| git-sage responses | Adapts explanations to your style |
| Daily report generation | Matches your preferred format |
| Task descriptions | Uses your phrasing patterns |

If no profile exists, these features fall back to standard AI output — no errors, just no personalization.

---

## Data Storage

| Store | Location | Contents |
|---|---|---|
| Local files | `DATA_DIR/learning/` | Samples (JSONL), profile (JSON), consent |
| MongoDB (optional) | `MONGODB_URI` | `communication_samples`, `user_profiles`, `learning_state` |
| Vector store | `DATA_DIR/learning/chroma/` | Embeddings for RAG (if AI tier installed) |
