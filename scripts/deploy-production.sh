#!/usr/bin/env sh
set -eu

ENV_FILE="${ENV_FILE:-.env.production}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing production environment file: $ENV_FILE" >&2
  exit 2
fi

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

echo "Validating production Compose configuration"
compose config >/dev/null

echo "Building API and web images"
compose build api web

echo "Applying forward database migrations"
compose run --rm migrate

echo "Replacing API container"
compose up -d --no-deps api

echo "Waiting for API readiness"
attempt=0
until compose exec -T api wget -q -O - http://127.0.0.1:8080/ready >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "API did not become ready; web was not replaced." >&2
    compose logs --tail=120 api >&2 || true
    exit 1
  fi
  sleep 2
done

echo "Replacing web container"
compose up -d --no-deps web

echo "Deployment containers"
compose ps

if command -v git >/dev/null 2>&1; then
  sha="$(git rev-parse HEAD 2>/dev/null || true)"
  if [ -n "$sha" ]; then
    echo "Deployed Git SHA: $sha"
  fi
fi
