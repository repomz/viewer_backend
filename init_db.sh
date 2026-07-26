#!/usr/bin/env bash

set -Eeuo pipefail

DB_NAME="${DB_NAME:-angio_db}"
DB_USER="${DB_USER:-angio_user}"
: "${DB_PASS:?DB_PASS is required}"

for identifier in "$DB_NAME" "$DB_USER"; do
  if [[ ! "$identifier" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]]; then
    printf 'Invalid PostgreSQL identifier: %s\n' "$identifier" >&2
    exit 1
  fi
done

printf 'Creating or updating PostgreSQL role %s and database %s...\n' \
  "$DB_USER" "$DB_NAME"

sudo -u postgres psql \
  --set ON_ERROR_STOP=1 \
  --set db_name="$DB_NAME" \
  --set db_user="$DB_USER" \
  --set db_pass="$DB_PASS" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN', :'db_user')
WHERE NOT EXISTS (
  SELECT 1 FROM pg_roles WHERE rolname = :'db_user'
)
\gexec

SELECT format('ALTER ROLE %I PASSWORD %L', :'db_user', :'db_pass')
\gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'db_name', :'db_user')
WHERE NOT EXISTS (
  SELECT 1 FROM pg_database WHERE datname = :'db_name'
)
\gexec

SELECT format('ALTER DATABASE %I OWNER TO %I', :'db_name', :'db_user')
\gexec
SQL

sudo -u postgres psql \
  --set ON_ERROR_STOP=1 \
  --dbname "$DB_NAME" \
  --set db_user="$DB_USER" <<'SQL'
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', current_database(), :'db_user')
\gexec

SELECT format('GRANT ALL ON SCHEMA public TO %I', :'db_user')
\gexec

SELECT format(
  'ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %I',
  :'db_user'
)
\gexec

SELECT format(
  'ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %I',
  :'db_user'
)
\gexec
SQL

printf 'PostgreSQL database %s is ready for role %s.\n' "$DB_NAME" "$DB_USER"
