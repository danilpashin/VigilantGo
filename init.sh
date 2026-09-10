#!/bin/bash
set -e

psql \
    -v ON_ERROR_STOP=1 \
    -v migrator_user="$MIGRATOR_DB_USER" \
    -v migrator_password="$MIGRATOR_DB_PASSWORD" \
    -v app_user="$APP_DB_USER" \
    -v app_password="$APP_DB_PASSWORD" \
    -v readonly_user="$READONLY_DB_USER" \
    -v readonly_password="$READONLY_DB_PASSWORD" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" <<'EOSQL'
CREATE ROLE :"migrator_user" LOGIN PASSWORD :'migrator_password';
CREATE ROLE :"app_user" LOGIN PASSWORD :'app_password';
CREATE ROLE :"readonly_user" LOGIN PASSWORD :'readonly_password';

GRANT ALL PRIVILEGES ON SCHEMA public TO :"migrator_user";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO :"migrator_user";
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO :"migrator_user";

GRANT USAGE ON SCHEMA public TO :"app_user";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO :"app_user";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO :"app_user";

GRANT USAGE ON SCHEMA public TO :"readonly_user";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO :"readonly_user";

ALTER DEFAULT PRIVILEGES FOR ROLE :"migrator_user" IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO :"app_user";

ALTER DEFAULT PRIVILEGES FOR ROLE :"migrator_user" IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO :"app_user";

ALTER DEFAULT PRIVILEGES FOR ROLE :"migrator_user" IN SCHEMA public
    GRANT SELECT ON TABLES TO :"readonly_user";
EOSQL