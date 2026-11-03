#!/usr/bin/env bash
set -euo pipefail

: "${SVAROG_IMAGE:?Set SVAROG_IMAGE to the server image to deploy}"

compose=(docker compose --env-file /opt/svarog/.env -f docker-compose.prod.yml)

"${compose[@]}" config --quiet
# Pull both images before touching running containers.
"${compose[@]}" pull
if ! "${compose[@]}" up --detach --no-build --pull never --wait --wait-timeout 300; then
  "${compose[@]}" ps --all
  exit 1
fi

"${compose[@]}" ps
printf 'Deployed %s\n' "$SVAROG_IMAGE"
