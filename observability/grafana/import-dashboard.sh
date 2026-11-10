#!/usr/bin/env sh
# Usage: GRAFANA_URL=http://localhost:22300 OVERWRITE=false sh observability/grafana/import-dashboard.sh
set -eu

url="${GRAFANA_URL:-http://localhost:22300}"
overwrite="${OVERWRITE:-false}"
case "$overwrite" in
  true|false) ;;
  *) echo 'OVERWRITE must be true or false' >&2; exit 2 ;;
esac

dashboard=$(cat "$(dirname -- "$0")/trip-service.json")
printf '{"dashboard":%s,"overwrite":%s}\n' "$dashboard" "$overwrite" |
  curl --fail-with-body --silent --show-error --proto '=http,https' \
    --noproxy '*' --connect-timeout 3 --max-time 15 \
    --header 'Content-Type: application/json' --data-binary @- \
    --url "${url%/}/api/dashboards/db"
printf '\n'
