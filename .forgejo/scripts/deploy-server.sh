#!/usr/bin/env bash
set -euo pipefail

: "${SVAROG_IMAGE:?Set SVAROG_IMAGE to the server image to deploy}"
: "${SVAROG_DEPLOY_DIR:?Set SVAROG_DEPLOY_DIR to an absolute host path}"

case "$SVAROG_DEPLOY_DIR" in
  /*) ;;
  *)
    echo "SVAROG_DEPLOY_DIR must be an absolute host path." >&2
    exit 1
    ;;
esac

compose=(docker compose --env-file "$SVAROG_DEPLOY_DIR/.env" -f docker-compose.prod.yml)

"${compose[@]}" config --quiet
# Pull both images before touching running containers.
"${compose[@]}" pull
if ! "${compose[@]}" up --detach --no-build --pull never --wait --wait-timeout 300; then
  "${compose[@]}" ps --all
  exit 1
fi

"${compose[@]}" ps
printf 'Deployed %s\n' "$SVAROG_IMAGE"
