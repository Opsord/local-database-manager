#!/bin/bash
set -e

# Start SQL Server in the background
/opt/mssql/bin/sqlservr &
sqlservr_pid=$!

# Locate sqlcmd (path varies across images)
SQLCMD_BIN="$(command -v sqlcmd 2>/dev/null || ls /opt/mssql-tools*/bin/sqlcmd 2>/dev/null | head -n1)"
if [ -z "$SQLCMD_BIN" ]; then
  echo "ERROR: sqlcmd not found; initialization scripts will be skipped."
  wait $sqlservr_pid
  exit 0
fi
echo "Using sqlcmd at: $SQLCMD_BIN"

SA_PASS="${SA_PASSWORD}"
DB_NAME="${SQLSERVER_DB}"
SCHEMA_NAME="${SQLSERVER_SCHEMA:-dbo}"

echo "Waiting for SQL Server to become available..."
until "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C -Q "SELECT 1" >/dev/null 2>&1; do
  sleep 1
done

echo "SQL Server is up. Running initialization scripts..."
for f in /init/*.sql; do
  [ -e "$f" ] || continue
  echo "Running $f"
  "$SQLCMD_BIN" -S localhost -U sa -P "$SA_PASS" -C \
    -v DB="$DB_NAME" SCHEMA="$SCHEMA_NAME" -i "$f" || echo "WARNING: $f failed"
done

echo "Initialization complete."

# Keep the container alive
wait $sqlservr_pid
