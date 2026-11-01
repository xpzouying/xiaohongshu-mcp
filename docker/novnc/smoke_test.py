#!/usr/bin/env python3
"""使用隔离容器检查桌面镜像，不登录账号或调用发布工具。"""

import base64
import json
import os
from pathlib import Path
import socket
import struct
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid


def docker(*args, check=True, timeout=30):
    result = subprocess.run(
        ["docker", *args], capture_output=True, text=True, timeout=timeout
    )
    if check and result.returncode:
        print(result.stdout + result.stderr, file=sys.stderr)
        result.check_returncode()
    return result


def ready(url):
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                return response.read()
        except (OSError, urllib.error.URLError):
            time.sleep(0.2)
    raise AssertionError(f"服务未就绪：{url}")


def check_rfb(port, origin="http://127.0.0.1", expected=101):
    # 实际完成 WebSocket/RFB 握手，确认 HTTP 页面背后有可用的桌面。
    with socket.create_connection(("127.0.0.1", port), timeout=10) as connection:
        key = base64.b64encode(os.urandom(16)).decode()
        origin_header = f"Origin: {origin}\r\n" if origin is not None else ""
        connection.sendall(
            (f"GET /websockify HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\n"
             "Upgrade: websocket\r\nConnection: Upgrade\r\n"
             f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n"
             f"{origin_header}"
             "Sec-WebSocket-Protocol: binary\r\n\r\n").encode()
        )

        def receive(size):
            data = b""
            while len(data) < size:
                part = connection.recv(size - len(data))
                if not part:
                    raise AssertionError("桌面连接提前关闭")
                data += part
            return data

        headers = b""
        while not headers.endswith(b"\r\n\r\n"):
            headers += receive(1)
        assert int(headers.split()[1]) == expected, headers
        if expected != 101:
            return
        pending = b""

        def read(size):
            nonlocal pending
            while len(pending) < size:
                first, second = receive(2)
                assert first & 15 == 2, first
                length = second & 127
                if length == 126:
                    length = struct.unpack(">H", receive(2))[0]
                elif length == 127:
                    length = struct.unpack(">Q", receive(8))[0]
                assert not second & 128
                pending += receive(length)
            value, pending = pending[:size], pending[size:]
            return value

        def send(data):
            mask = os.urandom(4)
            assert len(data) < 126
            connection.sendall(
                bytes([0x82, 0x80 | len(data)]) + mask
                + bytes(value ^ mask[index % 4] for index, value in enumerate(data))
            )

        assert read(12).startswith(b"RFB 003.")
        send(b"RFB 003.008\n")
        assert 1 in read(read(1)[0])
        send(b"\1")
        assert read(4) == b"\0" * 4
        send(b"\1")
        initial = read(24)
        assert struct.unpack(">HH", initial[:4]) == (1440, 1000)
        read(struct.unpack(">I", initial[20:24])[0])


def main(image):
    name = "xhs-novnc-test-" + uuid.uuid4().hex[:12]
    network = name + "-network"
    containers = []
    try:
        metadata = json.loads(docker("image", "inspect", image).stdout)[0]
        assert set(metadata["Config"]["ExposedPorts"]) == {"18060/tcp", "6080/tcp"}
        assert metadata["Config"]["Cmd"] == ["/app/app", "-headless=false"]
        docker("run", "--rm", "--entrypoint", "python3", image, "-c", """
from pathlib import Path
import subprocess
for name in ('libqt5webenginecore5', 'libqt5webenginewidgets5', 'fcitx5-module-cloudpinyin', 'openssh-server'):
    status = subprocess.run(['dpkg-query', '-W', '-f=${Status}', name], capture_output=True, text=True).stdout
    assert status != 'install ok installed', name
assert not list(Path('/var/lib/apt/lists').glob('*'))
assert not list(Path('/app/data').rglob('cookies.json'))
""")
        docker("network", "create", network)
        with tempfile.TemporaryDirectory(prefix="xhs-novnc-test-") as directory:
            media = Path(directory)
            containers.append(name)
            docker(
                "run", "-d", "--name", name, "--network", network,
                "--cap-drop=ALL", "--security-opt=no-new-privileges:true", "--shm-size=1g",
                "-e", "AUTH_TOKEN=isolated-smoke-test", "-p", "127.0.0.1::18060",
                "-e", "NOVNC_ALLOWED_ORIGINS=http://127.0.0.1",
                "-p", "127.0.0.1::6080", "--mount",
                f"type=bind,src={media},dst=/app/images,readonly", image,
            )

            def port(container_port):
                address = docker("port", name, f"{container_port}/tcp").stdout.strip()
                assert address.startswith("127.0.0.1:"), address
                return int(address.rsplit(":", 1)[1])

            api_port, desktop_port = port(18060), port(6080)

            def check_ready():
                assert json.loads(ready(f"http://127.0.0.1:{port(18060)}/health"))["success"]
                assert b"noVNC" in ready(f"http://127.0.0.1:{port(6080)}/vnc.html")
                check_rfb(port(6080))

            check_ready()
            check_rfb(desktop_port, "https://untrusted.example", 403)
            check_rfb(desktop_port, None, 403)
            try:
                urllib.request.urlopen(f"http://127.0.0.1:{api_port}/mcp", timeout=2)
                raise AssertionError("MCP 未拒绝匿名请求")
            except urllib.error.HTTPError as error:
                assert error.code == 401, error.code
            print("PASS 启动、MCP 鉴权、回环端口和 WebSocket/RFB 桌面", flush=True)
            print("PASS 拒绝跨站及缺少 Origin 的桌面连接", flush=True)

            (media / "newest.txt").write_text("created after startup\n")
            docker("exec", name, "python3", "-c", """
import errno
from pathlib import Path
assert Path('/app/images/newest.txt').read_text() == 'created after startup\\n'
try:
    Path('/app/images/write-attempt').touch()
except OSError as error:
    assert error.errno == errno.EROFS
else:
    raise AssertionError('素材目录可写')
""")
            print("PASS 宿主新增素材可见、容器不可写", flush=True)

            docker("cp", str(Path(__file__).with_name("input_test.py")), name + ":/tmp/input_test.py")
            result = docker("exec", name, "python3", "-u", "/tmp/input_test.py", timeout=90)
            print(result.stdout, end="", flush=True)

            docker("exec", name, "python3", "-c", """
import os, signal
from pathlib import Path
pids = [int(p.name) for p in Path('/proc').iterdir() if p.name.isdigit()
        and (p/'comm').exists() and (p/'comm').read_text().strip() == 'x11vnc']
assert pids
for pid in pids:
    os.kill(pid, signal.SIGTERM)
""")
            assert int(docker("wait", name).stdout) != 0
            docker("start", name)
            check_ready()
            docker("kill", "--signal", "KILL", name)
            docker("start", name)
            check_ready()
            print("PASS 桌面故障退出、重启与 SIGKILL 后显示锁恢复", flush=True)

            for status, expected in ((0, 1), (42, 42)):
                wrapper = media / f"fcitx-exit-{status}.sh"
                wrapper.write_text(f"#!/bin/sh\nexit {status}\n")
                wrapper.chmod(0o755)
                child = name + f"-component-{status}"
                containers.append(child)
                result = docker(
                    "run", "--name", child, "--network", "none", "--mount",
                    f"type=bind,src={wrapper},dst=/usr/local/bin/fcitx5,readonly", image,
                    check=False,
                )
                assert result.returncode == expected, result.stderr
            for status in (0, 7):
                child = name + f"-app-{status}"
                containers.append(child)
                result = docker("run", "--name", child, "--network", "none", image,
                                "/bin/sh", "-c", f"exit {status}", check=False)
                assert result.returncode == status, result.stderr
            print("PASS 组件提前退出不会漏检、MCP 退出码保留", flush=True)
    except Exception:
        for container in containers:
            logs = docker("logs", "--tail", "30", container, check=False)
            print(logs.stdout + logs.stderr, file=sys.stderr)
        raise
    finally:
        for container in containers:
            docker("rm", "-f", container, check=False)
        docker("network", "rm", network, check=False)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("用法：python3 docker/novnc/smoke_test.py <image>")
    main(sys.argv[1])
