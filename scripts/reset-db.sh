#!/usr/bin/env sh
# Rebuild the local database from nothing: drop -> goose migrations -> seeds.
# Destroys every row in the local `collection` database. Local dev only.
# See Documentation/seed-data.md.
set -eu
cd "$(dirname "$0")/.."

docker compose up -d --wait db
docker compose stop api
docker compose exec -T db psql -U collection -d postgres -v ON_ERROR_STOP=1 \
  -c 'DROP DATABASE IF EXISTS collection WITH (FORCE)' \
  -c 'CREATE DATABASE collection'

# The API runs migrations, then the reference seeds (and the dev seeds, since
# docker-compose sets SEED_DEV_DATA=true), before it starts serving.
docker compose up -d api
i=0
until curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 90 ]; then
    echo "api did not become healthy; check: docker compose logs api" >&2
    exit 1
  fi
  sleep 2
done
docker compose logs api | grep '"seeds applied"' | tail -1
