# syntax=docker/dockerfile:1.6
#
# Dockerfile.server — DevTrack Python Backend Server
#
# Packages the Python backend as a standalone deployable server.
# The devtrack Go binary connects to this server for AI and integration processing.
#
# Architecture:
#   devtrack (Go binary)  →  HTTPS POST /trigger/*  →  devtrack-server (this)
#   The server handles AI, integrations and webhooks.
#
# Build:
#   docker build -t devtrack-server .
#
# Run (local, with .env):
#   docker run --rm -p 8089:8089 \
#     --env-file .env \
#     -v $(pwd)/Data:/app/Data \
#     devtrack-server
#
# Run (docker compose):
#   docker compose up -d devtrack_server
#
# Then point the client at it:
#   DEVTRACK_SERVER_MODE=external  DEVTRACK_SERVER_URL=http://localhost:8089
#
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1
ENV PATH="/root/.local/bin:${PATH}"

RUN apt-get update && apt-get install -y --no-install-recommends \
        curl \
        git \
        sqlite3 \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install uv for dependency management
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

WORKDIR /app

# Install Python dependencies (cache layer).
# UV_PROJECT_ENVIRONMENT is what makes `uv sync` target /opt/venv; without it uv
# resolves the project venv to /app/.venv and the image ships without its deps.
ENV UV_PROJECT_ENVIRONMENT=/opt/venv
RUN --mount=type=cache,target=/root/.cache/uv \
    uv venv /opt/venv
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev

ENV PATH="/opt/venv/bin:${PATH}"
ENV VIRTUAL_ENV="/opt/venv"

# Copy backend source
COPY backend/ ./backend/

# Data directory for SQLite, logs, etc.
RUN mkdir -p /app/Data

# IPC port (Go daemon connects here) and webhook/API port
EXPOSE 35893 8089

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -sf http://localhost:8089/health || exit 1

CMD ["python", "-m", "backend.webhook_server"]
