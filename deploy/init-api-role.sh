#!/usr/bin/env bash
set -euo pipefail

psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set=ON_ERROR_STOP=1 \
    --set=api_password="$API_DATABASE_PASSWORD" <<'SQL'
CREATE ROLE reelcanon_api WITH LOGIN PASSWORD :'api_password';
GRANT CONNECT ON DATABASE reelcanon TO reelcanon_api;
GRANT USAGE ON SCHEMA public TO reelcanon_api;
ALTER DEFAULT PRIVILEGES FOR ROLE reelcanon IN SCHEMA public
    GRANT SELECT ON TABLES TO reelcanon_api;
ALTER DEFAULT PRIVILEGES FOR ROLE reelcanon IN SCHEMA public
    GRANT SELECT, USAGE ON SEQUENCES TO reelcanon_api;
SQL
