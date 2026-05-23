# syntax=docker/dockerfile:1.6
#
# Dockerfile.server — DevTrack Python Backend Server
#
# Packages the Python backend as a standalone deployable server.
# The devtrack Go binary connects to this server for AI and integration processing.
#
# Architecture:
#   devtrack (Go binary)  →  connects via IPC (localhost or remote)
#   devtrack-server (this)  ←  handles AI, integrations, webhooks, Telegram bot
#
# Build:
#   docker build -f Dockerfile.server -t devtrack-server .
#
# Run (local, with .env):
#   docker run --rm -p 35893:35893 -p 8089:8089 \
#     --env-file .env \
#     -v $(pwd)/Data:/app/Data \
#     devtrack-server
#
# Run (docker compose):
#   docker compose up   (see docker-compose.yml)
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

# Install Python dependencies (cache layer)
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv venv /opt/venv && \
    VIRTUAL_ENV=/opt/venv uv sync --frozen --no-dev

ENV PATH="/opt/venv/bin:${PATH}"
ENV VIRTUAL_ENV="/opt/venv"

# Copy backend source
COPY backend/ ./backend/
COPY python_bridge.py ./
COPY .env_sample ./

# Data directory for SQLite, logs, etc.
RUN mkdir -p /app/Data

# IPC port (Go daemon connects here) and webhook/API port
EXPOSE 35893 8089

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD python -c "import socket; s=socket.socket(); s.connect(('127.0.0.1', 35893)); s.close()" 2>/dev/null || exit 1

CMD ["python", "python_bridge.py"]
