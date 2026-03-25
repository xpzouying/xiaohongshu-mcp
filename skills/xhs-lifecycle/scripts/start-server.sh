#!/usr/bin/env bash
# Start xiaohongshu-mcp server if not already running.
# Usage: bash start-server.sh [repo_dir]
# Exit codes: 0 = server ready, 1 = failed to start

set -euo pipefail

# Default to repo root (two levels up from this script)
XHS_DIR="${1:-$(cd "$(dirname "$0")/../.." && pwd)}"
PORT="${XHS_PORT:-18060}"
PIDFILE="$XHS_DIR/.server.pid"
LOGFILE="$XHS_DIR/.server.log"

# --- detect local Chrome/Chromium (avoids rod downloading Chromium, which can be slow) ---
detect_chrome_bin() {
  if [ -n "${ROD_BROWSER_BIN:-}" ]; then
    echo "$ROD_BROWSER_BIN"
    return
  fi
  local candidates=(
    # macOS
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    "/Applications/Chromium.app/Contents/MacOS/Chromium"
    # Linux
    "/usr/bin/google-chrome"
    "/usr/bin/google-chrome-stable"
    "/usr/bin/chromium"
    "/usr/bin/chromium-browser"
    # Windows (Git Bash / WSL)
    # WSL paths may lack the executable bit, so we also check with -f below
    "/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"
    "/c/Program Files/Google/Chrome/Application/chrome.exe"
  )
  for c in "${candidates[@]}"; do
    # -x works on native platforms; -f is a fallback for WSL where .exe files
    # are accessible but may not have the executable permission bit set.
    if [ -x "$c" ] || [ -f "$c" ]; then
      echo "$c"
      return
    fi
  done
}

CHROME_BIN="$(detect_chrome_bin)"

# --- helpers ---
# Port check with cross-platform fallback: lsof (macOS/most Linux) -> ss (minimal Linux) -> curl
is_running() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -i ":${PORT}" -sTCP:LISTEN >/dev/null 2>&1
  elif command -v ss >/dev/null 2>&1; then
    ss -tlnp | grep -q ":${PORT} "
  else
    curl -s -o /dev/null --max-time 2 "http://127.0.0.1:${PORT}" 2>/dev/null
  fi
}

wait_ready() {
  local pid=$1
  local tries=0
  while [ $tries -lt 15 ]; do
    if is_running; then
      return 0
    fi
    # Detect early crash to avoid waiting the full 15 seconds
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "ERROR: server process (pid=$pid) exited unexpectedly"
      return 1
    fi
    sleep 1
    tries=$((tries + 1))
  done
  return 1
}

# --- main ---
if is_running; then
  echo "OK: xiaohongshu-mcp already running on port ${PORT}"
  exit 0
fi

if [ ! -x "$XHS_DIR/xiaohongshu-mcp" ]; then
  echo "ERROR: binary not found at $XHS_DIR/xiaohongshu-mcp"
  echo "Run: cd $XHS_DIR && go build -o xiaohongshu-mcp ."
  exit 1
fi

echo "Starting xiaohongshu-mcp on port ${PORT}..."
cd "$XHS_DIR"
if [ -n "$CHROME_BIN" ]; then
  echo "Using local browser: $CHROME_BIN"
  export ROD_BROWSER_BIN="$CHROME_BIN"
fi
nohup ./xiaohongshu-mcp -headless=true -port ":${PORT}" > "$LOGFILE" 2>&1 &
SERVER_PID=$!
echo $SERVER_PID > "$PIDFILE"

if wait_ready "$SERVER_PID"; then
  echo "OK: xiaohongshu-mcp started (pid=$SERVER_PID)"
  exit 0
else
  echo "ERROR: server did not become ready within 15s"
  echo "Check logs: $LOGFILE"
  exit 1
fi
