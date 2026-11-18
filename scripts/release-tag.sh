#!/usr/bin/env bash
set -euo pipefail

pattern='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
tag="${1:-}"

if [[ "$tag" == --latest ]]; then
  git ls-remote --refs --tags origin 'refs/tags/v*' \
    | cut -f2 | sed 's|^refs/tags/||' | grep -E "$pattern" | LC_ALL=C sort -V | tail -n 1
elif [[ "$tag" =~ $pattern ]]; then
  printf '%s\n' "${tag#v}"
else
  printf 'Expected a release tag in the form vX.Y.Z, got %q\n' "$tag" >&2
  exit 1
fi
