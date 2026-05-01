---
name: Local-first offline operation is Rule 0 — never breakable
description: Everything must work locally on a local model completely offline; client-server is optional, not a replacement
type: feedback
---

At any point it must ALWAYS be possible for a developer to run everything locally as this is the core idea for the tool. Client-server only comes into picture as an offering for people who need it. NO change should break the one rule that EVERYTHING should be possible to be run locally on a local model completely offline.

**Why:** This is the core premise of DevTrack. It was built for developers who want workflow automation without cloud dependency. Breaking this would make it just another SaaS tool.

**How to apply:**
- `DEVTRACK_SERVER_MODE=managed` (local subprocess) is the primary mode — never treat it as legacy or a fallback
- Ollama on localhost is a first-class LLM backend — always support it
- SQLite is the primary database — no feature should require PostgreSQL or MongoDB to function
- Never add a hard dependency on a remote URL, internet connection, or cloud service
- Every new feature must work in managed/local mode before considering remote/cloud support
- When implementing CS-1 (IPC→HTTP), the managed subprocess path must remain identical to today
- Degradation chain: cloud LLM unavailable → fall back to local Ollama; Ollama down → fall back to raw text; always degrade gracefully
