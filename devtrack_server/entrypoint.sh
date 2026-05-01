#!/bin/bash
# DevTrack Container Entrypoint
# Keeps container alive and allows devtrack daemon to run

set -e

# If a command was provided, execute it
if [ $# -gt 0 ]; then
    exec "$@"
fi

# Otherwise, run a simple sleep loop to keep container alive
# The daemon will be started via docker exec by the wrapper script
echo "DevTrack container ready. Waiting for daemon start..."
while true; do
    sleep 3600
done
