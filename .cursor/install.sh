#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ── System packages (PostgreSQL for Python server / managed mode) ─────────────
if ! command -v psql >/dev/null 2>&1; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    postgresql postgresql-client curl ca-certificates
fi

# ── uv (Python deps) on default PATH for non-interactive login shells ─────────
if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/install.sh | sh
  sudo ln -sf "${HOME}/.local/bin/uv" /usr/local/bin/uv
fi

# ── Go client binary ──────────────────────────────────────────────────────────
mkdir -p "${HOME}/.local/bin"
(
  cd "$REPO_ROOT/devtrack_client"
  go build -o "${HOME}/.local/bin/devtrack" .
)
sudo ln -sf "${HOME}/.local/bin/devtrack" /usr/local/bin/devtrack

# ── Python server dependencies ────────────────────────────────────────────────
(
  cd "$REPO_ROOT/devtrack_server"
  uv sync
)

# ── Managed-mode server path (monorepo dev without sparse clone) ─────────────
XDG_SERVER="${XDG_DATA_HOME:-$HOME/.local/share}/devtrack/server"
mkdir -p "$XDG_SERVER"
ln -sfn "$REPO_ROOT/devtrack_server" "$XDG_SERVER/devtrack_server"

# ── Local PostgreSQL role/database (idempotent) ───────────────────────────────
if command -v pg_isready >/dev/null 2>&1; then
  sudo service postgresql start 2>/dev/null || true
  for _ in $(seq 1 30); do
    if pg_isready -q 2>/dev/null; then break; fi
    sleep 1
  done
  if pg_isready -q 2>/dev/null; then
    sudo -u postgres psql -v ON_ERROR_STOP=0 -tc \
      "SELECT 1 FROM pg_roles WHERE rolname='devtrack'" | grep -q 1 ||
      sudo -u postgres psql -c "CREATE USER devtrack WITH PASSWORD 'devtrack' CREATEDB;"
    sudo -u postgres psql -v ON_ERROR_STOP=0 -tc \
      "SELECT 1 FROM pg_database WHERE datname='devtrack'" | grep -q 1 ||
      sudo -u postgres createdb -O devtrack devtrack
  fi
fi

# ── Server env for local development (sourced by start.sh; not pytest .env) ───
SERVER_ENV="$REPO_ROOT/devtrack_server/.env.cloud-agent"
if [[ ! -f "$SERVER_ENV" ]]; then
  cp "$REPO_ROOT/devtrack_server/.env_sample" "$SERVER_ENV"
  sed -i \
    -e "s|^PROJECT_ROOT=.*|PROJECT_ROOT=$REPO_ROOT/devtrack_server|" \
    -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=devtrack|" \
    -e "s|^POSTGRES_URL=.*|POSTGRES_URL=postgresql://devtrack:devtrack@127.0.0.1:5432/devtrack|" \
    -e "s|^ADMIN_SECRET_KEY=.*|ADMIN_SECRET_KEY=cloud-agent-dev-secret-key-32ch|" \
    -e "s|^DEVTRACK_API_KEY=.*|DEVTRACK_API_KEY=cloud-agent-dev-api-key|" \
    "$SERVER_ENV"
fi
# Remove legacy .env if present so pytest keeps SQLite isolation (see CI).
rm -f "$REPO_ROOT/devtrack_server/.env"

# ── Monorepo client .env (registered via devtrack.conf on first use) ──────────
CLIENT_ENV="$REPO_ROOT/.env.cloud-agent"
if [[ ! -f "$CLIENT_ENV" ]]; then
  cat > "$CLIENT_ENV" <<EOF
PROJECT_ROOT=$REPO_ROOT/devtrack_client
DEVTRACK_HOME=$REPO_ROOT/devtrack_client
DEVTRACK_WORKSPACE=$REPO_ROOT
WORKSPACES_FILE=$REPO_ROOT/devtrack_client/Data/workspaces.yaml
DATABASE_DIR=$REPO_ROOT/devtrack_client/Data/db
LOG_DIR=$REPO_ROOT/devtrack_client/Data/logs
PID_DIR=$REPO_ROOT/devtrack_client/Data/pids
CONFIG_DIR_PATH=$REPO_ROOT/devtrack_client/Data/configs
LEARNING_DIR_PATH=$REPO_ROOT/devtrack_client/Data/learning
CLI_BINARY_NAME=devtrack
CONFIG_FILE_NAME=config.yaml
DATABASE_FILE_NAME=devtrack.db
PID_FILE_NAME=daemon.pid
LOG_FILE_NAME=daemon.log
LEARNING_DIR_NAME=learning
CONFIG_DIR_NAME=.devtrack
CLI_APP_NAME=DevTrack
CLI_DAEMON_NAME=devtrack
DEVTRACK_SERVER_MODE=external
DEVTRACK_SERVER_URL=http://127.0.0.1:8089
DEVTRACK_TLS=false
DEVTRACK_API_KEY=cloud-agent-dev-api-key
IPC_HOST=127.0.0.1
IPC_PORT=35893
IPC_CONNECT_TIMEOUT_SECS=5
IPC_RETRY_DELAY_MS=500
DEVTRACK_SERVER_HTTP_PORT=35894
PROMPT_INTERVAL=30
WORK_HOURS_ONLY=false
WORK_START_HOUR=0
WORK_END_HOUR=23
TIMEZONE=UTC
LOG_LEVEL=info
AUTO_SYNC=false
OUTPUT_TYPE=console
DAILY_REPORT_TIME=18:00
WEEKLY_REPORT_DAY=Friday
SEND_ON_TRIGGER=false
SEND_DAILY_SUMMARY=false
TEAMS_MENTION_USER=false
LEARNING_DEFAULT_DAYS=30
SERVER_EVENT_SYNC_ENABLED=false
TICKET_SYNC_ON_START=false
QUEUE_POLL_INTERVAL_SECS=60
HEALTH_CHECK_INTERVAL_SECS=60
VOICE_SYNC_INTERVAL_HOURS=24
DEVTRACK_AUTO_ACCEPT_TERMS=1
PYTHONIOENCODING=utf-8
OLLAMA_HOST=http://127.0.0.1:11434
OLLAMA_MODEL=llama3.2
GIT_SAGE_PROVIDER=ollama
GIT_SAGE_DEFAULT_MODEL=llama3.2
HTTP_TIMEOUT_SHORT_SECS=5
HTTP_TIMEOUT=30
HTTP_TIMEOUT_LONG=60
SQLITE_BUSY_TIMEOUT_MS=5000
EOF
  mkdir -p "$REPO_ROOT/devtrack_client/Data/db" \
    "$REPO_ROOT/devtrack_client/Data/logs" \
    "$REPO_ROOT/devtrack_client/Data/pids" \
    "$REPO_ROOT/devtrack_client/Data/configs" \
    "$REPO_ROOT/devtrack_client/Data/learning" \
    "$REPO_ROOT/devtrack_client/Data/tls"
fi

mkdir -p "${HOME}/.devtrack"
echo "ENV_FILE=$CLIENT_ENV" > "${HOME}/.devtrack/devtrack.conf"

echo "DevTrack cloud install complete."
