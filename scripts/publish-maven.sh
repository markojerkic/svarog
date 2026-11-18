#!/usr/bin/env bash
set -euo pipefail

bundle=${1:?Expected path to the signed Central bundle}
: "${MAVEN_CENTRAL_USERNAME:?Missing Sonatype token username}" "${MAVEN_CENTRAL_TOKEN:?Missing Sonatype token password}"
auth=$(printf '%s:%s' "$MAVEN_CENTRAL_USERNAME" "$MAVEN_CENTRAL_TOKEN" | base64 -w0)
deployment=$(printf 'Authorization: Bearer %s\n' "$auth" |
  curl --fail-with-body --silent --show-error --max-time 120 --header @- \
    --form "bundle=@$bundle;type=application/octet-stream" \
    'https://central.sonatype.com/api/v1/publisher/upload?publishingType=AUTOMATIC')
if [[ ! "$deployment" =~ ^[[:xdigit:]]{8}(-[[:xdigit:]]{4}){3}-[[:xdigit:]]{12}$ ]]; then
  printf 'Sonatype returned an invalid deployment ID.\n' >&2
  exit 1
fi
printf 'Submitted Sonatype deployment %s for automatic publishing (%s).\n' \
  "$deployment" 'https://central.sonatype.com/publishing/deployments'
