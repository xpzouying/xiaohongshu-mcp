"""
Chrome launcher with CDP remote debugging support.

Manages a dedicated Chrome instance for Xiaohongshu publishing:
- Detects if Chrome is already listening on the debug port
- Launches Chrome with a dedicated user-data-dir for login persistence
- Waits for the debug port to become available
- Supports headless mode for automated publishing without GUI
- Supports switching between headless and headed mode (e.g. for login)
- Supports multiple accounts with separate profile directories
"""

import os
import sys
import time
import socket
import subprocess
import platform
import signal
from typing import Optional

CDP_PORT = 9222
PROFILE_DIR_NAME = "XiaohongshuProfile"
STARTUP_TIMEOUT = 15  # seconds to wait for Chrome to start

# Track the Chrome process we launched so we can kill it later
_chrome_process: subprocess.Popen | None = None
# Track the current account being used
_current_account: Optional[str] = None


def get_chrome_path() -> str:
    """Find Chrome executable on Windows."""
    candidates = []

    # Standard install locations
    for env_var in ("PROGRAMFILES", "PROGRAMFILES(X86)", "LOCALAPPDATA"):
        base = os.environ.get(env_var, "")
        if base:
            candidates.append(os.path.join(base, "Google", "Chrome", "Application", "chrome.exe"))

    for path in candidates:
        if os.path.isfile(path):
            return path

    # Fallback: check PATH
    import shutil
    found = shutil.which("chrome") or shutil.which("chrome.exe")
    if found:
        return found

    raise FileNotFoundError(
        "Chrome not found. Please install Google Chrome or set its path manually."
    )


def get_user_data_dir(account: Optional[str] = None) -> str:
    """
    Return the Chrome profile directory path for a given account.

    Args:
        account: Account name. If None, uses the default account from account_manager.

    Returns:
        Path to the Chrome user-data-dir for this account.
    """
    try:
        from account_manager import get_profile_dir
        return get_profile_dir(account)
    except ImportError:
        # Fallback if account_manager not available
        local_app_data = os.environ.get("LOCALAPPDATA", "")
        if not local_app_data:
            local_app_data = os.path.expanduser("~")
        return os.path.join(local_app_data, "Google", "Chrome", PROFILE_DIR_NAME)


def is_port_open(port: int, host: str = "127.0.0.1") -> bool:
    """Check if a TCP port is accepting connections."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.settimeout(1)
        try:
            s.connect((host, port))
            return True
        except (ConnectionRefusedError, socket.timeout, OSError):
            return False


def launch_chrome(port: int = CDP_PORT, headless: bool = False, account: Optional[str] = None) -> subprocess.Popen | None:
    """
    Launch Chrome with remote debugging enabled.

    Args:
        port: CDP remote debugging port.
        headless: If True, launch Chrome in headless mode (no GUI window).
        account: Account name to use. If None, uses the default account.

    Returns the Popen object if a new process was started, or None if Chrome
    was already running on the target port.
    """
    global _chrome_process, _current_account

    if is_port_open(port):
        print(f"[chrome_launcher] Chrome already running on port {port}.")
        return None

    chrome_path = get_chrome_path()
    user_data_dir = get_user_data_dir(account)
    _current_account = account

    cmd = [
        chrome_path,
        f"--remote-debugging-port={port}",
        f"--user-data-dir={user_data_dir}",
        "--no-first-run",
        "--no-default-browser-check",
    ]

    if headless:
        cmd.append("--headless=new")

    mode_label = "headless" if headless else "headed"
    account_label = account or "default"
    print(f"[chrome_launcher] Launching Chrome ({mode_label}, account: {account_label})...")
    print(f"  executable : {chrome_path}")
    print(f"  profile dir: {user_data_dir}")
    print(f"  debug port : {port}")

    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    _chrome_process = proc

    # Wait for the debug port to become available
    deadline = time.time() + STARTUP_TIMEOUT
    while time.time() < deadline:
        if is_port_open(port):
            print(f"[chrome_launcher] Chrome is ready on port {port}.")
            return proc
        time.sleep(0.5)

    print(
        f"[chrome_launcher] WARNING: Chrome started but port {port} not responding "
        f"after {STARTUP_TIMEOUT}s. It may still be initializing.",
        file=sys.stderr,
    )
    return proc


def kill_chrome(port: int = CDP_PORT):
    """
    Kill the Chrome instance on the given debug port.

    Tries multiple strategies:
    1. Send CDP Browser.close command via HTTP
    2. Terminate the tracked subprocess
    3. Kill by port on Windows (taskkill)
    """
    global _chrome_process

    # Strategy 1: CDP Browser.close
    try:
        import requests
        resp = requests.get(f"http://127.0.0.1:{port}/json/version", timeout=2)
        if resp.ok:
            ws_url = resp.json().get("webSocketDebuggerUrl")
            if ws_url:
                import websockets.sync.client as ws_client
                ws = ws_client.connect(ws_url)
                ws.send('{"id":1,"method":"Browser.close"}')
                try:
                    ws.recv(timeout=2)
                except Exception:
                    pass
                ws.close()
                print("[chrome_launcher] Sent Browser.close via CDP.")
    except Exception:
        pass

    # Wait briefly for Chrome to shut down
    time.sleep(1)

    # Strategy 2: Terminate tracked subprocess
    if _chrome_process and _chrome_process.poll() is None:
        try:
            _chrome_process.terminate()
            _chrome_process.wait(timeout=5)
            print("[chrome_launcher] Terminated tracked Chrome process.")
        except Exception:
            try:
                _chrome_process.kill()
            except Exception:
                pass
    _chrome_process = None

    # Strategy 3: Windows taskkill by port (fallback)
    if sys.platform == "win32" and is_port_open(port):
        try:
            result = subprocess.run(
                ["netstat", "-ano"],
                capture_output=True, text=True, timeout=5
            )
            for line in result.stdout.splitlines():
                if f":{port}" in line and "LISTENING" in line:
                    pid = line.strip().split()[-1]
                    subprocess.run(
                        ["taskkill", "/F", "/PID", pid],
                        capture_output=True, timeout=5
                    )
                    print(f"[chrome_launcher] Killed process {pid} via taskkill.")
                    break
        except Exception:
            pass

    # Wait for port to be released
    deadline = time.time() + 5
    while time.time() < deadline:
        if not is_port_open(port):
            return
        time.sleep(0.5)

    if is_port_open(port):
        print(f"[chrome_launcher] WARNING: port {port} still open after kill attempt.",
              file=sys.stderr)


def restart_chrome(port: int = CDP_PORT, headless: bool = False, account: Optional[str] = None) -> subprocess.Popen | None:
    """
    Kill the current Chrome instance and relaunch with the specified mode.

    Useful for switching between headless and headed mode (e.g. when login
    is needed during a headless session), or switching accounts.

    Args:
        port: CDP remote debugging port.
        headless: If True, relaunch in headless mode.
        account: Account name to use. If None, uses the default account.

    Returns the Popen object for the new Chrome process.
    """
    account_label = account or "default"
    print(f"[chrome_launcher] Restarting Chrome ({'headless' if headless else 'headed'}, account: {account_label})...")
    kill_chrome(port)
    time.sleep(1)
    return launch_chrome(port, headless=headless, account=account)


def ensure_chrome(port: int = CDP_PORT, headless: bool = False, account: Optional[str] = None) -> bool:
    """
    Ensure Chrome is running with remote debugging on the given port.

    Args:
        port: CDP remote debugging port.
        headless: If True, launch in headless mode when starting a new instance.
            If Chrome is already running, this parameter is ignored.
        account: Account name to use. If None, uses the default account.

    Returns True if Chrome is available, False otherwise.
    """
    if is_port_open(port):
        return True
    try:
        launch_chrome(port, headless=headless, account=account)
        return is_port_open(port)
    except FileNotFoundError as e:
        print(f"[chrome_launcher] Error: {e}", file=sys.stderr)
        return False


def get_current_account() -> Optional[str]:
    """Get the name of the currently active account."""
    return _current_account


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description="Chrome Launcher for CDP")
    parser.add_argument("--headless", action="store_true", help="Launch in headless mode")
    parser.add_argument("--kill", action="store_true", help="Kill the running Chrome instance")
    parser.add_argument("--restart", action="store_true", help="Restart Chrome")
    parser.add_argument("--account", help="Account name to use (default: default account)")
    args = parser.parse_args()

    if args.kill:
        kill_chrome()
        print("[chrome_launcher] Chrome killed.")
    elif args.restart:
        restart_chrome(headless=args.headless, account=args.account)
        print("[chrome_launcher] Chrome restarted.")
    elif ensure_chrome(headless=args.headless, account=args.account):
        print("[chrome_launcher] Chrome is ready for CDP connections.")
    else:
        print("[chrome_launcher] Failed to start Chrome.", file=sys.stderr)
        sys.exit(1)
