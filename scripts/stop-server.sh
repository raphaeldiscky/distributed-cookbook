#!/usr/bin/env bash
# Kill whatever process is listening on a TCP port.
#
# Task's mvdan/sh doesn't implement `kill` as a builtin, so we use
# /bin/kill explicitly (external binary, always available on Linux/macOS).
#
# Usage:
#   scripts/stop-server.sh <port>

set -euo pipefail

PORT="${1:-}"
if [ -z "${PORT}" ]; then
    echo "usage: $0 <port>" >&2
    exit 1
fi

PIDS=$(lsof -ti "tcp:${PORT}" 2>/dev/null || true)
if [ -z "${PIDS}" ]; then
    echo "nothing listening on :${PORT}"
else
    echo "killing on :${PORT} → ${PIDS}"
    # shellcheck disable=SC2086  # PIDS is intentionally word-split
    /bin/kill ${PIDS}
fi
