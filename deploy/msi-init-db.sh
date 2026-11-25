#!/usr/bin/env bash
# One-time on MSI: create bills role/database on shared-postgres and set DATABASE_URL in deploy.env.
set -euo pipefail

SECRETS_DIR="${SECRETS_DIR:-/home/tuf/dev/shared/bill-sharing/secrets}"
DEPLOY_ENV="$SECRETS_DIR/deploy.env"
DB_USER="${DB_USER:-bills}"
DB_NAME="${DB_NAME:-bills}"

if [[ -f "$DEPLOY_ENV" ]]; then
  # shellcheck disable=SC1090
  set -a
  source "$DEPLOY_ENV"
  set +a
fi

if [[ -z "${DB_PASS:-}" ]]; then
  if [[ -n "${POSTGRES_PASSWORD:-}" ]]; then
    DB_PASS="$POSTGRES_PASSWORD"
  else
    DB_PASS="$(openssl rand -hex 16)"
  fi
fi

DB_PASS_SQL="${DB_PASS//\'/\'\'}"

if ! docker exec shared-postgres psql -U postgres -At -c "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1; then
  printf '%s\n' "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASS_SQL}';" \
    | docker exec -i shared-postgres psql -U postgres -v ON_ERROR_STOP=1
  echo "created role ${DB_USER}"
fi

if docker exec shared-postgres psql -U postgres -At -c "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
  echo "database ${DB_NAME} already exists on shared-postgres"
else
  printf '%s\n' \
    "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};" \
    "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};" \
    | docker exec -i shared-postgres psql -U postgres -v ON_ERROR_STOP=1

  printf '%s\n' \
    "GRANT ALL ON SCHEMA public TO ${DB_USER};" \
    "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};" \
    | docker exec -i shared-postgres psql -U postgres -d "${DB_NAME}" -v ON_ERROR_STOP=1
  echo "created database ${DB_NAME}"
fi

DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@shared-postgres:5432/${DB_NAME}?sslmode=disable"

mkdir -p "$SECRETS_DIR"
chmod 700 "$(dirname "$SECRETS_DIR")" "$SECRETS_DIR"

if [[ -f "$DEPLOY_ENV" ]]; then
  cp "$DEPLOY_ENV" "${DEPLOY_ENV}.bak.$(date +%Y%m%d%H%M%S)"
fi

if [[ -f "$DEPLOY_ENV" ]] && grep -q '^DATABASE_URL=' "$DEPLOY_ENV"; then
  sed -i "s|^DATABASE_URL=.*|DATABASE_URL=${DATABASE_URL}|" "$DEPLOY_ENV"
else
  echo "DATABASE_URL=${DATABASE_URL}" >>"$DEPLOY_ENV"
fi
sed -i '/^POSTGRES_/d' "$DEPLOY_ENV" 2>/dev/null || true
chmod 600 "$DEPLOY_ENV"

echo "OK: DATABASE_URL set in $DEPLOY_ENV (host shared-postgres)"