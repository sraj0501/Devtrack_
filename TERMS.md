# DevTrack — Terms of Service & End User License Agreement

**Version 1.0 — Effective March 31, 2026**

---

## 1. Acceptance

By installing, running, or using DevTrack in any form (local binary, self-hosted
server, or cloud-hosted SaaS), you agree to these Terms. If you do not agree,
do not use DevTrack.

Your acceptance is recorded locally in `Data/license/acceptance.json`. For
team/cloud deployments, acceptance is also recorded server-side linked to your
account.

---

## 2. Licence Tiers

| Tier       | Users       | Cost | Notes                              |
|------------|-------------|------|------------------------------------|
| Personal   | 1           | Free | Personal, non-commercial use only  |
| Team       | 2 – 10      | Free | Self-hosted; all core features     |
| Enterprise | 11+         | Paid | Commercial licence required        |
| SaaS Cloud | Any         | Paid | Hosted by DevTrack; see pricing    |

The user count for Team tier is determined by the number of distinct user
accounts that have accepted these Terms and connected to the same DevTrack
workspace within the last 90 days.

**Single-user offline mode is always free and never requires network access.**

---

## 3. What DevTrack Collects (Local Mode)

In local / self-hosted mode with telemetry **disabled** (default):

- DevTrack sends no product telemetry.
- Git observation and the client database remain local.
- Data is sent only to integrations and providers that you configure, such as a PM platform,
  email service, remote DevTrack server, or cloud LLM. Ollama remains the default AI path.

---

## 4. Telemetry (Opt-in Only)

If you explicitly enable telemetry (`devtrack login` then opt-in during setup):

**What is collected:**
- Commands run (e.g., `devtrack start`, `devtrack git commit`) — no arguments
- Feature usage counts (e.g., NLP parsing invoked, LLM provider used)
- Error codes and stack trace summaries (no file content, no commit messages)
- DevTrack version, OS type, Go/Python runtime version

**What is NEVER collected:**
- Source code, file contents, or diff contents
- Commit messages or work update text
- Git repository names or paths
- Personal data (name, email) beyond your login identifier
- API keys, credentials, or environment variable values

**How to disable:**
```bash
devtrack telemetry off     # Disable immediately
devtrack logout            # Logout also disables telemetry
```

Telemetry data is retained for 12 months and used solely for product
improvement. It is not sold or shared with third parties.

---

## 5. Authentication & Login

Login is **optional** for personal use. It is required for:
- Team licence seat tracking (2–10 users)
- Enterprise licence validation
- Cloud / SaaS hosted deployments
- Telemetry (opt-in)

Authentication uses email-based magic links. No password is stored.
Your session token is stored locally in `Data/license/session.json` and
is valid for 90 days.

**Offline guarantee**: If you are logged in and lose network access, DevTrack
continues to function using the locally cached session. Licence validation
falls back to local cache for up to 90 days.

---

## 6. Data & Privacy

- Client-owned work state is stored locally in SQLite under the configured data directory. A
  managed or external Python server uses PostgreSQL for server-owned persistence.
- Personalization / "Talk Like You" collection begins only after explicit enablement. Server-owned
  samples use the configured learning store (PostgreSQL fallback or optional MongoDB), and the
  optional RAG tier stores embeddings in ChromaDB.
- DevTrack does not access your source code beyond git metadata (branch names,
  commit SHAs, file-change counts) unless you invoke a feature that requires it
  (e.g., `devtrack sage` git agent).
- Learning data can be removed with `devtrack learning-reset`. Client SQLite and server PostgreSQL
  data have separate operator-controlled retention and deletion lifecycles.

---

## 7. Acceptable Use

You may not use DevTrack to:
- Circumvent access controls or security measures of any system
- Collect or process data in violation of applicable law (GDPR, CCPA, etc.)
- Resell or sub-license DevTrack without a commercial licence
- Remove or obscure licence notices or attributions

---

## 8. Updates to These Terms

DevTrack may update these Terms. You will be notified on next launch when Terms
change. Continued use after notification constitutes acceptance of the updated
Terms.

---

## 9. Disclaimer & Limitation of Liability

DevTrack is provided "as is" without warranty. The authors are not liable for
any damages arising from use of the software, including but not limited to data
loss, business interruption, or security incidents.

---

## 10. Contact

- Licence enquiries: license@devtrack.dev
- Privacy & data requests: privacy@devtrack.dev
- General support: https://devtrack.dev
