#!/usr/bin/env bash
# Check xiaohongshu-mcp server status.
# Usage: bash check-status.sh [repo_dir]
# Outputs: SERVER_UP or SERVER_DOWN with details.

set -euo pipefail

XHS_DIR="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
PORT=18060
PIDFILE="$XHS_DIR/.server.pid"

if lsof -i ":${PORT}" -sTCP:LISTEN >/dev/null 2>&1; then
  PID=$(lsof -t -i ":${PORT}" -sTCP:LISTEN 2>/dev/null || echo "unknown")
  echo "SERVER_UP pid=${PID} port=${PORT}"
else
  echo "SERVER_DOWN port=${PORT}"
  if [ -f "$PIDFILE" ]; then
    echo "stale pidfile: $(cat "$PIDFILE")"
  fi
fi
