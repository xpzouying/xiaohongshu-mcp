#!/usr/bin/env bash
set -Eeuo pipefail

# dbus-run-session 提供共享会话，浏览器与输入法必须继承同一组环境。
export DISPLAY=:99 XDG_RUNTIME_DIR=/tmp/xhs-runtime
mkdir -p "$XDG_RUNTIME_DIR" "$XDG_CONFIG_HOME/fcitx5" "$XDG_CONFIG_HOME/gtk-3.0"
chmod 700 "$XDG_RUNTIME_DIR"
if [[ ! -f "$XDG_CONFIG_HOME/fcitx5/profile" ]]; then
    cp /usr/local/share/xhs-fcitx5-profile "$XDG_CONFIG_HOME/fcitx5/profile"
fi
# 不覆盖用户已有设置，只在首次运行添加素材入口。
touch "$XDG_CONFIG_HOME/gtk-3.0/bookmarks"
if ! grep -q '^file:///app/images\( \|$\)' "$XDG_CONFIG_HOME/gtk-3.0/bookmarks"; then
    printf 'file:///app/images images\n' >> "$XDG_CONFIG_HOME/gtk-3.0/bookmarks"
fi
printf 'Gtk/IMModule "fcitx"\n' > "$XDG_RUNTIME_DIR/xsettingsd.conf"

pids=()
cleanup() {
    trap - EXIT TERM INT
    if (( ${#pids[@]} )); then
        kill "${pids[@]}" 2>/dev/null || true
        wait "${pids[@]}" 2>/dev/null || true
    fi
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT

# SIGKILL 不执行退出清理；只移除已经没有 X 服务持有的固定显示锁。
python3 - <<'PY'
from pathlib import Path
lock = Path('/tmp/.X99-lock')
if lock.exists():
    pid = int(lock.read_text().strip())
    process = Path(f'/proc/{pid}/comm')
    if process.exists() and process.read_text().strip() in {'Xvfb', 'Xorg', 'X'}:
        raise SystemExit('显示 :99 已被其他 X 服务占用')
    lock.unlink()
    Path('/tmp/.X11-unix/X99').unlink(missing_ok=True)
PY

Xvfb "$DISPLAY" -screen 0 1440x1000x24 -nolisten tcp &
pids+=("$!")
ready=false
for ((i=0; i<50; i++)); do
    if xdpyinfo -display "$DISPLAY" >/dev/null 2>&1; then ready=true; break; fi
    if ! kill -0 "${pids[0]}" 2>/dev/null; then break; fi
    sleep 0.1
done
if [[ "$ready" != true ]]; then
    echo "Xvfb 启动失败" >&2
    exit 1
fi

openbox --sm-disable &
pids+=("$!")
fcitx5 &
pids+=("$!")
xsettingsd -c "$XDG_RUNTIME_DIR/xsettingsd.conf" &
pids+=("$!")
# VNC 仅在容器回环监听；Caps Lock 状态参与键码转换。
x11vnc -display "$DISPLAY" -rfbport 5900 -localhost -forever -shared -nopw -capslock &
pids+=("$!")
websockify --web /usr/share/novnc 6080 localhost:5900 &
pids+=("$!")
"$@" &
app_pid=$!
pids+=("$app_pid")

# 任一桌面组件退出都让容器失败，交给 restart 策略恢复，避免假健康。
# 定期检查全部子进程，避免 wait -n 漏掉进入等待前已退出的进程。
while true; do
    for pid in "${pids[@]}"; do
        if ! kill -0 "$pid" 2>/dev/null; then
            status=0
            wait "$pid" || status=$?
            if [[ "$pid" != "$app_pid" && "$status" == 0 ]]; then status=1; fi
            exit "$status"
        fi
    done
    sleep 1
done
