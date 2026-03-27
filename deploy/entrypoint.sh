#!/bin/sh
set -e

# Start API server in background
sailbox-api &
API_PID=$!

# Wait for API to be ready
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Start Caddy (foreground)
caddy run --config /etc/caddy/Caddyfile &
CADDY_PID=$!

# Graceful shutdown
shutdown() {
    kill "$CADDY_PID" 2>/dev/null
    kill "$API_PID" 2>/dev/null
    wait "$API_PID" 2>/dev/null
    exit 0
}

trap shutdown SIGTERM SIGINT

# Wait for either process to exit
wait -n "$API_PID" "$CADDY_PID" 2>/dev/null || true
shutdown
