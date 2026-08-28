# DevTrack website and wiki

This directory contains the static public site deployed at `devtrack.cloud`.

## Surfaces

- `wiki/index.html` — product homepage
- `wiki/download.html` — release download and installation entry point
- `wiki/wiki.html` — current user documentation with stable hash anchors
- `wiki/privacy.html` — current implementation-level privacy explanation
- `wiki/docs/` — Markdown references used by contributors and documentation reviews
- `wiki/install.sh` / `wiki/install.ps1` — release installers

The public narrative follows: standup outcome → local agent memory → pending-queue and local-first
trust. The latest public release and unreleased `dev` capabilities must always be distinguished.

## Local preview

```bash
cd devtrack_wiki
python3 -m http.server 8000
```

Open `http://localhost:8000/wiki/`.

## Validation

```bash
python3 check_inline_js.py
sh -n wiki/install.sh
```

Also verify:

- every homepage/footer hash exists in `wiki.html`;
- release asset names match `.github/workflows/release.yml`;
- client commands match `devtrack help`/`devtrack_client/main.go`;
- server routes and configuration match `devtrack_server/backend/`;
- no real credentials, invented usage claims, or unsupported compatibility claims appear.

## Source of truth

GitHub `sraj0501/Devtrack_` is the sole repository source. `PRODUCT_BIBLE.md` defines product
invariants, `docs/ARCHITECTURE.md` defines the client/server boundary, and
`Data/agent_logs/project_board.md` is the task ledger.

## License

The site documents the [DevTrack Community License](../LICENSE), not MIT.
