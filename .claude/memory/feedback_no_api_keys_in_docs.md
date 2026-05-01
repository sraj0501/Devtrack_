---
name: No API keys in docs or example files
description: Never write real or example API key values in markdown, documentation, or .env_sample files
type: feedback
---

Never write API keys, tokens, PATs, or secrets — real or fabricated — in any markdown file, documentation, or example config file (e.g. `.env_sample`, `*.md`).

**Why:** Two secrets (a Telegram bot token and an Azure DevOps PAT) were committed into markdown files and had to be scrubbed from git history with `git filter-branch`. Even example-looking values must not appear — they look real enough to confuse secret scanners and set a bad precedent.

**How to apply:**
- In docs: use only generic placeholders like `<your-api-key>`, `<your-pat>`, `<your-bot-token>`
- In `.env_sample`: leave values blank or use `<placeholder>` — never fill in a value even as an "example"
- Before writing any credential-adjacent content in a doc, ask: "could this be mistaken for or used as a real secret?" If yes, replace with a placeholder
- When reviewing diffs before commit, scan markdown changes for long alphanumeric strings that could be tokens
