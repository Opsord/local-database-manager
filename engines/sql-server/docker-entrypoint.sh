#!/bin/bash
set -Eeuo pipefail

# Validate required variables
: "${SA_PASSWORD:?SA_PASSWORD environment variable is required}"
: "${SQLSERVER_DB:?SQLSERVER_DB environment variable is required}"

SA_PASS="${SA_PASSWORD}"
DB_NAME="${SQLSERVER_DB}"
SCHEMA_NAME="${SQLSERVER_SCHEMA:-dbo}"

# Start SQL Server in the background
/opt/mssql/bin/sqlservr &
sqlservr_pid=$!

# Signal handler for graceful shutdown
cleanup() {
    echo "Caught shutdown signal. Forwarding SIGTERM to SQL Server (PID $sqlservr_pid)..."
    kill -TERM "$sqlservr_pid" 2>/dev/null || true
    wait "$sqlservr_pid" 2>/dev/null || true
}
trap cleanup SIGTERM SIGINT

# Locate sqlcmd binary safely across various base images
SQLCMD_BIN="$(command -v sqlcmd 2>/dev/null || true)"
if [ -z "$SQLCMD_BIN" ]; then
    for candidate in /opt/mssql-tools*/bin/sqlcmd; do
        if [ -x "$candidate" ]; then
            SQLCMD_BIN="$candidate"
            break
        fi
    done
fi

if [ -z "$SQLCMD_BIN" ]; then
    echo "ERROR: sqlcmd binary not found. Initialization scripts cannot be executed." >&2
    cleanup
    exit 1
fi
echo "Using sqlcmd at: $SQLCMD_BIN"

# Bounded readiness probe (max 60 seconds)
echo "Waiting for SQL Server to become available (max 60s)..."
MAX_ATTEMPTS=60
attempt=1
until "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C -Q "SELECT 1" >/dev/null 2>&1; do
    if [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
        echo "ERROR: SQL Server failed to become available after ${MAX_ATTEMPTS} seconds." >&2
        cleanup
        exit 1
    fi
    attempt=$((attempt + 1))
    sleep 1
done

# Execute initialization scripts safely
echo "SQL Server is ready. Running initialization scripts..."
if [ -d "/init" ]; then
    for f in /init/*.sql; do
        [ -f "$f" ] || continue
        echo "Executing initialization script: $f"
        "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C \
            -v DB="$DB_NAME" SCHEMA="$SCHEMA_NAME" -i "$f" || echo "WARNING: Script $f returned non-zero exit status"
    done
fi

echo "Initialization complete. Server running."

# Wait for background sqlservr process
wait "$sqlservr_pid"
