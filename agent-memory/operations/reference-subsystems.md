---
name: Subsystem references
description: Non-obvious Telegram, RAG, and Azure DevOps settings
type: reference
---

**Telegram:** Go bot starts with `TELEGRAM_ENABLED=true`; token/allow-list use centralized config. Commands: `/start /help /status /logs /health /trigger /pause /resume /stop /restart /reload /commits /queue`; queue messages provide approve/reject/edit callbacks.
**RAG:** optional ChromaDB personalization uses `PERSONALIZATION_RAG_ENABLED` and `PERSONALIZATION_CHROMA_DIR`; local embeddings require `ollama pull nomic-embed-text`.
**Azure:** `.env` stores only `AZURE_DEVOPS_PAT` (Work Items R/W + Code R). `workspaces.yaml` stores `pm_org`, `pm_project`, `pm_username`, and optional `pm_api_url`.
**Azure quirks:** WIQL date is `YYYY-MM-DD`; PR approval searches project pull requests and treats reviewer vote `>=10` as approved.
**Notifier incident rule:** Go Telegram/Slack constructors return the `Notifier` interface so disabled implementations cannot create typed-nil poller panics.
