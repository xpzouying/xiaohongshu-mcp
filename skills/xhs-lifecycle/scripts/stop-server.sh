#!/usr/bin/env bash
# Stop xiaohongshu-mcp server gracefully.
# Usage: bash stop-server.sh [repo_dir]
# Exit codes: 0 = stopped or not running, 1 = failed to stop

set -euo pipefail

XHS_DIR="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
PORT="${XHS_PORT:-18060}"
PIDFILE="$XHS_DIR/.server.pid"

# Try to find PID from pidfile or lsof
PID=""
if [ -f "$PIDFILE" ]; then
  PID=$(cat "$PIDFILE")
fi
if [ -z "$PID" ] && command -v lsof >/dev/null 2>&1; then
  PID=$(lsof -t -i ":${PORT}" -sTCP:LISTEN 2>/dev/null || true)
fi

if [ -z "$PID" ]; then
  echo "OK: xiaohongshu-mcp is not running"
  rm -f "$PIDFILE"
  exit 0
fi

echo "Stopping xiaohongshu-mcp (pid=$PID)..."
kill "$PID" 2>/dev/null || true

# Wait up to 5 seconds for graceful shutdown
tries=0
while [ $tries -lt 5 ]; do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "OK: stopped"
    rm -f "$PIDFILE"
    exit 0
  fi
  sleep 1
  tries=$((tries + 1))
done

# Force kill if still alive
kill -9 "$PID" 2>/dev/null || true
rm -f "$PIDFILE"
echo "OK: force stopped"
