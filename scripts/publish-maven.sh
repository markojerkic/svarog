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
printf 'Sonatype deployment: %s (%s)\n' "$deployment" 'https://central.sonatype.com/publishing/deployments'

deadline=$((SECONDS + 900))
while (( SECONDS < deadline )); do
  status=$(printf 'Authorization: Bearer %s\n' "$auth" |
    curl --fail-with-body --silent --show-error --retry 2 --retry-max-time 30 --max-time 20 \
      --header @- --request POST "https://central.sonatype.com/api/v1/publisher/status?id=$deployment")
  state=$(jq -er '.deploymentState' <<< "$status")
  case "$state" in
    PUBLISHED)
      printf 'Deployment %s published to Maven Central.\n' "$deployment"
      exit 0
      ;;
    FAILED)
      printf 'Deployment %s failed:\n' "$deployment" >&2
      jq '.errors' <<< "$status" >&2
      exit 1
      ;;
    PENDING|VALIDATING|VALIDATED|PUBLISHING) sleep 10 ;;
    *)
      printf 'Unexpected Sonatype deployment state: %s\n' "$state" >&2
      exit 1
      ;;
  esac
done
printf 'Timed out waiting for deployment %s. Check its status in the Central Portal before rerunning.\n' "$deployment" >&2
exit 1
