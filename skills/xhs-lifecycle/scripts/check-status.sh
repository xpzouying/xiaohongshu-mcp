#!/usr/bin/env bash
# Check xiaohongshu-mcp server status.
# Usage: bash check-status.sh [repo_dir]
# Outputs: SERVER_UP or SERVER_DOWN with details.

set -euo pipefail

XHS_DIR="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
PORT="${XHS_PORT:-18060}"
PIDFILE="$XHS_DIR/.server.pid"

# Port check with cross-platform fallback: lsof -> ss -> curl
is_listening() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -i ":${PORT}" -sTCP:LISTEN >/dev/null 2>&1
  elif command -v ss >/dev/null 2>&1; then
    ss -tlnp | grep -q ":${PORT} "
  else
    curl -s -o /dev/null --max-time 2 "http://127.0.0.1:${PORT}" 2>/dev/null
  fi
}

if is_listening; then
  PID=""
  if command -v lsof >/dev/null 2>&1; then
    PID=$(lsof -t -i ":${PORT}" -sTCP:LISTEN 2>/dev/null || true)
  fi
  echo "SERVER_UP pid=${PID:-unknown} port=${PORT}"
else
  echo "SERVER_DOWN port=${PORT}"
  if [ -f "$PIDFILE" ]; then
    echo "stale pidfile: $(cat "$PIDFILE")"
  fi
fi
