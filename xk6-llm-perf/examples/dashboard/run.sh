#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo"
binary=${K6_BINARY:-$repo/dist/k6}
version=$("$binary" version)
case "$version" in
  *k6/x/llm-perf*) ;;
  *) echo "dashboard: k6 binary does not contain k6/x/llm-perf" >&2; exit 1 ;;
esac
output=${DASHBOARD_OUTPUT_DIR:-"$HOME/tmp/xk6-llm-perf/$(date +%Y%m%d)-dashboard"}
mkdir -p "$output"
export K6_WEB_DASHBOARD=true
export K6_WEB_DASHBOARD_HOST=127.0.0.1
export K6_WEB_DASHBOARD_PORT=${K6_WEB_DASHBOARD_PORT:-5665}
export K6_WEB_DASHBOARD_PERIOD=${K6_WEB_DASHBOARD_PERIOD:-1s}
export K6_WEB_DASHBOARD_EXPORT="$output/report.html"
export XK6_DASHBOARD_CONFIG="$repo/examples/dashboard/config.json"
export SUMMARY_FILE="$output/summary.json"
exec "$binary" run "$repo/examples/dashboard/load.js" "$@"
