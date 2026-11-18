#!/usr/bin/env bash
set -euo pipefail

# HEAD is the commit even when the pushed ref is an annotated tag.
sha=$(git rev-parse HEAD)
deadline=$((SECONDS + 900))
while (( SECONDS < deadline )); do
  run=$(gh run list --repo "$GITHUB_REPOSITORY" --workflow test.yml \
    --branch master --event push --commit "$sha" --limit 1 \
    --json status,conclusion,url)
  if [[ "$(jq -r '.[0].status' <<< "$run")" == completed ]]; then
    conclusion=$(jq -r '.[0].conclusion' <<< "$run")
    printf 'Test on %s: %s (%s)\n' "$sha" "$conclusion" "$(jq -r '.[0].url' <<< "$run")"
    [[ "$conclusion" == success ]] || exit 1
    exit 0
  fi
  sleep 10
done
printf 'No completed master push Test run for %s within 15 minutes.\n' "$sha" >&2
exit 1
