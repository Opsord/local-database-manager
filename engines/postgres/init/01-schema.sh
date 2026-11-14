#!/bin/bash
set -Eeuo pipefail

SCHEMA_NAME="${POSTGRES_SCHEMA:-public}"
ESCAPED_SCHEMA="${SCHEMA_NAME//\"/\"\"}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-postgres}"

psql -v ON_ERROR_STOP=1 --username "$DB_USER" --dbname "$DB_NAME" <<-EOSQL
    CREATE SCHEMA IF NOT EXISTS "$ESCAPED_SCHEMA";
EOSQL
