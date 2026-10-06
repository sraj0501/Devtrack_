#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if command -v pg_isready >/dev/null 2>&1; then
  if ! pg_isready -q 2>/dev/null; then
    sudo service postgresql start
  fi
  for _ in $(seq 1 60); do
    if pg_isready -q; then break; fi
    sleep 1
  done
  if ! pg_isready -q; then
    echo "PostgreSQL did not become ready" >&2
    exit 1
  fi
  echo "PostgreSQL is ready"
fi

# Optional: Ollama for local LLM (install is heavy; start only if present)
if command -v ollama >/dev/null 2>&1; then
  if ! curl -sf "http://127.0.0.1:11434/api/tags" >/dev/null 2>&1; then
    ollama serve >/tmp/ollama.log 2>&1 &
    for _ in $(seq 1 30); do
      if curl -sf "http://127.0.0.1:11434/api/tags" >/dev/null 2>&1; then break; fi
      sleep 1
    done
  fi
  echo "Ollama endpoint checked"
fi

SERVER_ENV="$REPO_ROOT/devtrack_server/.env.cloud-agent"
if [[ -f "$SERVER_ENV" ]]; then
  if ! curl -sf "http://127.0.0.1:8089/health" >/dev/null 2>&1; then
    tmux -f /exec-daemon/tmux.portal.conf has-session -t devtrack_server 2>/dev/null ||
      tmux -f /exec-daemon/tmux.portal.conf new-session -d -s devtrack_server -c "$REPO_ROOT/devtrack_server" \
        "bash -lc 'set -a && source .env.cloud-agent && set +a && exec uv run python -m backend.webhook_server'"
    for _ in $(seq 1 60); do
      if curl -sf "http://127.0.0.1:8089/health" >/dev/null 2>&1; then break; fi
      sleep 1
    done
  fi
  if curl -sf "http://127.0.0.1:8089/health" >/dev/null 2>&1; then
    echo "DevTrack Python server is ready on :8089"
  else
    echo "Warning: Python server did not respond on :8089 (see tmux session devtrack_server)" >&2
  fi
fi

echo "DevTrack start script finished (PostgreSQL + server when configured)."
