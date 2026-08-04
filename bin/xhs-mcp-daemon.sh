#!/usr/bin/env bash
# xiaohongshu-mcp daemon — lives with the MCP project (not any business module).
#
# HTTP MCP on 127.0.0.1:18060. cwd MUST be the project root so cookies.json
# resolves correctly.
#
# State (pids/logs) defaults to ~/.xiaohongshu-mcp/ so the git working tree
# stays clean. Override with XHS_MCP_STATE_HOME.
#
# Usage:
#   ./bin/xhs-mcp-daemon.sh start|stop|restart|status|health|logs|cleanup-rod

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# project root = parent of bin/
XHS_MCP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="${XHS_MCP_BIN:-$XHS_MCP_DIR/xiaohongshu-mcp-darwin-arm64}"
# fallback name used by some builds
if [ ! -x "$BIN" ] && [ -x "$XHS_MCP_DIR/xiaohongshu-mcp" ]; then
	BIN="$XHS_MCP_DIR/xiaohongshu-mcp"
fi

STATE_HOME="${XHS_MCP_STATE_HOME:-$HOME/.xiaohongshu-mcp}"
LOG_DIR="$STATE_HOME/logs"
PID_DIR="$STATE_HOME/pids"
PID_FILE="$PID_DIR/xiaohongshu-mcp.pid"
LOG_FILE="$LOG_DIR/xiaohongshu-mcp.log"
PORT=18060

mkdir -p "$LOG_DIR" "$PID_DIR"
# chrome profile already uses ~/.xiaohongshu-mcp/chrome-profile when configured that way

if [ -t 1 ]; then RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'; else RED=; GREEN=; YELLOW=; NC=; fi
log_info()  { echo "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo "${RED}[ERROR]${NC} $*" >&2; }

port_pid() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1; }

do_start() {
	local existing
	existing="$(port_pid "$PORT" || true)"
	if [ -n "$existing" ]; then
		log_info "xiaohongshu-mcp already running on :$PORT (pid $existing)"
		return 0
	fi
	if [ ! -x "$BIN" ]; then
		log_error "binary not found/executable: $BIN"
		log_error "set XHS_MCP_BIN or build in $XHS_MCP_DIR"
		return 1
	fi
	if [ ! -f "$XHS_MCP_DIR/cookies.json" ]; then
		log_warn "cookies.json missing in $XHS_MCP_DIR — login may be required"
	fi
	# 显式有头 + 仅回环，避免旧二进制默认 headless/全网卡漂移
	# XHS_READ_ONLY=1 时禁用写工具（分析侧推荐）
	local listen="127.0.0.1:${PORT}"
	local headless_flag="${XHS_HEADLESS:-false}"
	log_info "Starting xiaohongshu-mcp on ${listen} headless=${headless_flag} (cwd=$XHS_MCP_DIR)"
	# cwd is a hard dependency for relative cookies.json
	(
		cd "$XHS_MCP_DIR"
		# shellcheck disable=SC2086
		nohup env \
			COOKIES_PATH="${COOKIES_PATH:-$XHS_MCP_DIR/cookies.json}" \
			XHS_READ_ONLY="${XHS_READ_ONLY:-}" \
			XHS_RISK_STREAK_LIMIT="${XHS_RISK_STREAK_LIMIT:-}" \
			XHS_FP_SEED="${XHS_FP_SEED:-}" \
			XHS_PROXY="${XHS_PROXY:-}" \
			"$BIN" \
			-headless="${headless_flag}" \
			-port "${listen}" \
			>>"$LOG_FILE" 2>&1 &
		echo $! >"$PID_FILE"
	)
	sleep 2
	if [ -n "$(port_pid "$PORT" || true)" ]; then
		log_info "xiaohongshu-mcp started (pid $(cat "$PID_FILE"))"
	else
		log_error "failed to start — see $LOG_FILE"
		return 1
	fi
}

do_stop() {
	local stopped=0 pid
	if [ -f "$PID_FILE" ]; then
		pid="$(cat "$PID_FILE" 2>/dev/null || true)"
		if [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null; then
			kill "$pid" 2>/dev/null || true
			sleep 0.5
			kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
			stopped=1
		fi
		rm -f "$PID_FILE"
	fi
	pid="$(port_pid "$PORT" || true)"
	if [ -n "${pid:-}" ]; then
		kill -9 "$pid" 2>/dev/null || true
		stopped=1
	fi
	[ "$stopped" = 1 ] && log_info "xiaohongshu-mcp stopped" || log_info "xiaohongshu-mcp was not running"
	# 常驻浏览器退出后偶发 rod 临时 profile 残留
	cleanup_rod || true
}

do_status() {
	local p rod
	p="$(port_pid "$PORT" || true)"
	echo "xiaohongshu-mcp  project=$XHS_MCP_DIR  state=$STATE_HOME"
	if [ -n "$p" ]; then
		printf "  :%s  ${GREEN}RUNNING${NC}  pid %s  bin=%s\n" "$PORT" "$p" "$BIN"
	else
		printf "  :%s  ${RED}STOPPED${NC}  start: %s start\n" "$PORT" "$0"
	fi
	rod="$(pgrep -f 'rod/user-data' 2>/dev/null | wc -l | tr -d ' ')"
	echo "  rod-Chrome procs: $rod  (run cleanup-rod if orphans pile up after use)"
}

do_health() {
	if [ -z "$(port_pid "$PORT" || true)" ]; then
		log_error "xiaohongshu-mcp not running (:$PORT)"
		return 1
	fi
	if curl -s --max-time 3 "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
		log_info "xiaohongshu-mcp healthy (:$PORT /health)"
		return 0
	fi
	log_warn "listening but /health not OK"
	return 1
}

cleanup_rod() {
	# rod leakless only reaps when the daemon dies; long-lived daemon leaves orphans
	local n
	n="$(pgrep -f 'rod/user-data' 2>/dev/null | wc -l | tr -d ' ')"
	if [ "${n:-0}" = "0" ]; then
		log_info "no rod/user-data Chrome processes"
		return 0
	fi
	log_info "reaping $n rod/user-data process group(s)..."
	pkill -f 'rod/user-data' 2>/dev/null || true
	sleep 0.5
	n="$(pgrep -f 'rod/user-data' 2>/dev/null | wc -l | tr -d ' ')"
	log_info "remaining rod procs: $n"
}

case "${1:-status}" in
	start)   do_start ;;
	stop)    do_stop ;;
	restart) do_stop; sleep 1; do_start ;;
	status)  do_status ;;
	health|health-check) do_health ;;
	logs)    tail -f "$LOG_FILE" ;;
	cleanup-rod|cleanup) cleanup_rod ;;
	*)
		cat <<EOF
Usage: $0 {start|stop|restart|status|health|logs|cleanup-rod}

Project cwd: $XHS_MCP_DIR  (required for cookies.json)
Binary:      $BIN
State:       $STATE_HOME
Listen:      127.0.0.1:$PORT
EOF
		exit 1
		;;
esac
