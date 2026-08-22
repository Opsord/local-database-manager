#!/bin/bash
set -Eeuo pipefail

SCHEMA_NAME="${POSTGRES_SCHEMA:-public}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-postgres}"

psql -v ON_ERROR_STOP=1 --username "$DB_USER" --dbname "$DB_NAME" <<-EOSQL
    CREATE SCHEMA IF NOT EXISTS "$SCHEMA_NAME";
EOSQL
