# syntax=docker/dockerfile:1.6
# Multi-stage Dockerfile for DevTrack
# Stage 1: Build Go application
FROM golang:1.24-bookworm AS go-builder

# Install build dependencies (build-essential provides gcc, libc, make)
RUN apt-get update && apt-get install -y --no-install-recommends \
        git \
        build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy Go module files
COPY devtrack-bin/go.mod devtrack-bin/go.sum ./
RUN go mod download

# Copy Go source code
COPY devtrack-bin/ ./

# Build the binary
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o devtrack .

# Stage 2: Base Python environment with cached apt/uv layers
FROM python:3.12-slim AS python-base

ENV PATH="/root/.local/bin:${PATH}"

RUN --mount=type=cache,target=/var/lib/apt \
    --mount=type=cache,target=/var/cache/apt \
    apt-get update && apt-get install -y --no-install-recommends \
        curl \
        git \
        sqlite3 \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# Stage 3: Install Python dependencies once
FROM python-base AS python-deps

RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system --no-cache-dir \
        spacy \
        en-core-web-sm@https://github.com/explosion/spacy-models/releases/download/en_core_web_sm-3.8.0/en_core_web_sm-3.8.0-py3-none-any.whl \
        dateparser \
        sentence-transformers \
        scikit-learn \
        fuzzywuzzy \
        python-Levenshtein \
        ollama

# Stage 4: Runtime environment
FROM python-base AS runtime

# Copy Python deps from cached stage
COPY --from=python-deps /usr/local /usr/local

# Create app user
RUN useradd -m -u 1000 devtrack && \
    mkdir -p /home/devtrack/.local/share/devtrack && \
    mkdir -p /home/devtrack/.config/devtrack

# Set working directory
WORKDIR /app

# Copy built binary from builder stage
COPY --from=go-builder /build/devtrack /usr/local/bin/devtrack
RUN chmod +x /usr/local/bin/devtrack

# Copy Python backend and entrypoints
COPY backend/ /app/backend/
COPY python_bridge.py entrypoint.sh /app/
COPY pyproject.toml /app/

# Make entrypoint executable
RUN chmod +x /app/entrypoint.sh

# Set up environment
ENV PATH="/home/devtrack/.local/bin:${PATH}"
ENV PYTHONUNBUFFERED=1
ENV DEVTRACK_DATA_DIR=/home/devtrack/.local/share/devtrack
ENV DEVTRACK_CONFIG_DIR=/home/devtrack/.config/devtrack

# Change ownership to app user
RUN chown -R devtrack:devtrack /app /home/devtrack

# Switch to non-root user
USER devtrack

# Volumes for persistent data
VOLUME ["/home/devtrack/.local/share/devtrack", "/home/devtrack/.config/devtrack", "/workspace"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD devtrack --version || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/devtrack"]
CMD ["--help"]
