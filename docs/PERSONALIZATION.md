# Personalization — "Talk Like You"

DevTrack can learn your communication style from your Teams messages and generate work updates, commit messages, and reports in your voice.

---

## How It Works

DevTrack uses a **two-signal personalization system**:

1. **Profile-based style instruction** — Fast, always available. Describes your style in abstract terms: formality level, message length preference, emoji usage, common phrases.

2. **RAG few-shot examples** — Retrieves actual past messages you wrote that are semantically similar to the current context, and shows them to the LLM as "write like this" examples. Much more effective than abstract descriptions for capturing voice nuance.

Both signals are automatically injected into every LLM prompt across the system — commit messages, work update descriptions, reports, task creation. If no profile exists, prompts are unchanged. Everything degrades gracefully.

---

## Setup

> **Prerequisite — `ai` tier**: Personalization depends on `spacy`, `sentence-transformers`, and `chromadb`, which are part of the optional `ai` dependency tier. Before following the steps below, ensure the tier is installed:
> ```bash
> devtrack-server enable ai
> devtrack restart
> ```
> Run `devtrack-server features` to confirm `✓ ai` is shown.

### Step 1: Enable Learning (One-time)

```bash
devtrack enable-learning
```

This:
1. Asks for your consent
2. Authenticates with Microsoft Graph (requires Teams/Outlook access)
3. Collects your Teams messages from the last 30 days (default) as training examples
4. Builds an initial communication profile

### Step 2: Install the RAG Embedding Model

For the few-shot example feature (optional but recommended). The `ai` tier must already be installed (Step 0 above) before pulling the embedding model:

```bash
ollama pull nomic-embed-text
```

Then in `.env`:

```env
PERSONALIZATION_RAG_ENABLED=true
PERSONALIZATION_EMBED_MODEL=nomic-embed-text
PERSONALIZATION_RAG_K=3        # Number of examples to inject per prompt
```

### Step 3: Keep It Updated

```bash
devtrack learning-sync          # Collect only new messages since last sync
devtrack learning-sync --full   # Force full 30-day re-collection
```

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PERSONALIZATION_RAG_ENABLED` | `true` | Enable RAG few-shot examples |
| `PERSONALIZATION_EMBED_MODEL` | `nomic-embed-text` | Ollama embedding model |
| `PERSONALIZATION_RAG_K` | `3` | Number of examples to inject per prompt |
| `PERSONALIZATION_CHROMA_DIR` | `DATA_DIR/learning/chroma` | ChromaDB storage path |
| `LEARNING_DEFAULT_DAYS` | `30` | Days of history to collect |
| `LEARNING_CRON_ENABLED` | `false` | Enable daily automatic sync |
| `LEARNING_CRON_SCHEDULE` | `"0 20 * * *"` | Cron schedule for daily sync |

---

## CLI Commands

```bash
devtrack enable-learning [days]   # Enable learning (consent + initial data collection)
devtrack learning-sync            # Delta sync (only new messages since last run)
devtrack learning-sync --full     # Force full re-sync
devtrack learning-status          # Show consent status and sample count
devtrack show-profile             # Display your learned communication profile
devtrack test-response <text>     # Generate a personalized response without auth
devtrack revoke-consent           # Delete all data and revoke consent
devtrack learning-reset           # Wipe everything and start fresh
```

### Learning Cron

```bash
devtrack learning-setup-cron     # Install/update daily cron from LEARNING_CRON_SCHEDULE
devtrack learning-cron-status    # Show cron entry and schedule
devtrack learning-remove-cron    # Remove the cron entry
```

---

## Testing Personalization

After enabling learning, test whether it's working:

```bash
# Generate a personalized response for any text (no auth required)
devtrack test-response "Completed the authentication module and fixed 3 bugs"

# Show your current learned profile
devtrack show-profile
```

The profile shows inferred style characteristics like:
- Formality level
- Typical message length
- Emoji usage preference
- Common phrases and vocabulary

---

## Data Storage

| Data | Location | Contents |
|------|----------|----------|
| Consent file | `Data/learning/consent.json` | User email, Azure AD object ID |
| Communication samples | MongoDB `communication_samples` collection | Teams message trigger→response pairs |
| Style profile | MongoDB `user_profiles` collection | Computed style characteristics |
| Sync state | MongoDB `learning_state` collection | Delta sync timestamp |
| RAG embeddings | `Data/learning/chroma/` | ChromaDB vector store |

**MongoDB is required** for the full personalization pipeline. If `MONGODB_URI` is not set, samples are stored in local files at `Data/learning/`.

---

## Privacy

- Learning is **fully opt-in** — `enable-learning` asks for explicit consent
- All data is stored **locally** (in your MongoDB instance or local files)
- No data is sent to external AI services — embedding and inference use local Ollama models
- You can delete everything at any time: `devtrack learning-reset` or `devtrack revoke-consent`

---

## Troubleshooting

**`enable-learning` fails with auth error:**
- Microsoft Teams integration requires an Azure AD app with `Chat.Read` permissions
- Self-hosted Teams: ensure Graph API access is configured
- See `backend/msgraph_python/` for Graph setup details

**`learning-sync` always reports 0 new samples:**
- Run `devtrack learning-sync --full` to ignore the delta state and re-collect everything
- Check that your Teams account has message history

**Personalization has no effect on prompts:**
- Run `devtrack learning-status` — check that sample count > 0 and profile exists
- Run `devtrack show-profile` to confirm profile is populated
- For RAG: verify `nomic-embed-text` is pulled in Ollama

**RAG not working:**
- Verify: `uv run python -c "import chromadb; print('OK')"`
- Verify: `curl http://localhost:11434/api/tags | grep nomic-embed-text`
- Set `PERSONALIZATION_RAG_ENABLED=true` in `.env`
