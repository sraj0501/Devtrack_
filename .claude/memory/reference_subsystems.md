---
name: Subsystem references
description: Config/env details for Telegram, RAG personalization, and Azure DevOps not covered by CLAUDE.md
type: reference
---

_gitsage behaviour (Go-native; squash via `reset --soft`, JSON mode, Groq model choice) and the personalization design are documented in CLAUDE.md — not duplicated here. Only the settings you'd otherwise have to dig for:_

## Telegram bot (`devtrack_client/internal/telegram/`)

Starts automatically with the daemon when `TELEGRAM_ENABLED=true`; implements `notify.Notifier`. Access control: `TELEGRAM_ALLOWED_CHAT_IDS` (`config_env.go`).
Commands: `/start /help /status /logs /health /trigger /pause /resume /stop /restart /reload /commits`.

## RAG personalization

Prerequisite the setup docs bury: `ollama pull nomic-embed-text`. Env: `PERSONALIZATION_RAG_ENABLED`, `PERSONALIZATION_CHROMA_DIR`.

## Azure DevOps

`.env` holds the secret only — `AZURE_DEVOPS_PAT`, scopes **Work Items R/W + Code R**.
`workspaces.yaml` holds every non-secret value: `pm_org`, `pm_project`, `pm_username`, `pm_api_url` (**blank = dev.azure.com**).
