#!/bin/bash
# Copy every recipe's Grafana dashboard JSON into the provisioned dashboards dir.
# Grafana polls that dir every 10s (see dashboards.yaml), so new recipes are
# discovered without editing any shared file.
set -eu

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="$REPO_ROOT/deployments/docker-compose/monitoring/grafana/dashboards"

mkdir -p "$TARGET"

shopt -s nullglob
for dash in "$REPO_ROOT"/recipes/*/grafana/*.json; do
    cp "$dash" "$TARGET/"
    echo "synced $(basename "$dash")"
done
