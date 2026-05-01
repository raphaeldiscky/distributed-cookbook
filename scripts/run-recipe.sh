#!/usr/bin/env bash
# Run a recipe's server with hot reload via air.
#
# Air watches recipes/<name>/ and pkg/ for .go changes, rebuilds, and
# restarts the server. It sends SIGINT on restart so the server's
# graceful-shutdown path drains in-flight requests before exit.
# kill_delay=3s keeps dev iteration snappy (shorter than the server's
# own 10s shutdown timeout — slow requests get dropped during dev,
# which is the right tradeoff).
#
# Usage:
#   scripts/run-recipe.sh <recipe-name>

set -euo pipefail

RECIPE="${1:-}"
if [ -z "${RECIPE}" ]; then
    echo "usage: $0 <recipe-name>" >&2
    exit 1
fi

exec air \
    --build.cmd "go build -o ./tmp/${RECIPE}-server ./recipes/${RECIPE}/cmd/server" \
    --build.bin "./tmp/${RECIPE}-server" \
    --build.include_dir "recipes/${RECIPE},pkg" \
    --build.include_ext "go" \
    --build.exclude_dir "tmp,vendor,node_modules,deployments,docs,.git,.husky" \
    --build.kill_delay 3s \
    --build.delay 500 \
    --build.stop_on_error true \
    --misc.clean_on_exit true
