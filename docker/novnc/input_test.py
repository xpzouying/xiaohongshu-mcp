#!/usr/bin/env python3
"""在容器内用真实 RFB 键鼠事件检查输入法及系统文件窗口。"""

import json
import os
from pathlib import Path
import re
import select
import socket
import struct
import subprocess
import tempfile
import time
import urllib.parse


def main():
    environment = os.environ.copy()
    for entry in Path('/proc').iterdir():
        try:
            if entry.name.isdigit() and (entry / 'comm').read_text().strip() == 'fcitx5':
                for variable in (entry / 'environ').read_bytes().split(b'\0'):
                    if b'=' in variable:
                        key, value = variable.split(b'=', 1)
                        environment[key.decode()] = value.decode()
                break
        except FileNotFoundError:
            continue
    else:
        raise AssertionError('Fcitx5 未运行')

    browser = next(Path('/app/cache/xiaohongshu-mcp/browser').glob('*/chrome'))
    incoming, writer = os.pipe()
    reader, outgoing = os.pipe()

    def attach_pipes():
        os.dup2(incoming, 3)
        os.dup2(outgoing, 4)

    with tempfile.TemporaryDirectory(prefix='xhs-input-test-') as profile:
        with open('/tmp/xhs-input-test.log', 'wb') as log:
            process = subprocess.Popen(
                [str(browser), '--no-sandbox', '--no-first-run', '--disable-dev-shm-usage',
                 '--remote-debugging-pipe', '--user-data-dir=' + profile],
                env=environment, stdin=subprocess.DEVNULL, stdout=log, stderr=log,
                pass_fds=(incoming, writer, outgoing), preexec_fn=attach_pipes,
            )
            os.close(incoming)
            os.close(outgoing)
            pending = b''
            sequence = 0

            def cdp(method, params=None, session=None):
                nonlocal sequence, pending
                sequence += 1
                message = {'id': sequence, 'method': method, 'params': params or {}}
                if session:
                    message['sessionId'] = session
                os.write(writer, json.dumps(message).encode() + b'\0')
                deadline = time.monotonic() + 20
                while time.monotonic() < deadline:
                    while b'\0' in pending:
                        raw, pending = pending.split(b'\0', 1)
                        response = json.loads(raw)
                        if response.get('id') == sequence:
                            if 'error' in response:
                                raise AssertionError(response['error'])
                            return response['result']
                    if select.select([reader], [], [], 0.2)[0]:
                        data = os.read(reader, 65536)
                        if not data:
                            raise AssertionError('浏览器调试连接关闭')
                        pending += data
                raise TimeoutError(method)

            with socket.create_connection(('127.0.0.1', 5900), timeout=10) as connection:
                def receive(size):
                    data = b''
                    while len(data) < size:
                        part = connection.recv(size - len(data))
                        if not part:
                            raise AssertionError('RFB 连接关闭')
                        data += part
                    return data

                try:
                    receive(12)
                    connection.sendall(b'RFB 003.008\n')
                    assert 1 in receive(receive(1)[0])
                    connection.sendall(b'\1')
                    assert receive(4) == b'\0' * 4
                    connection.sendall(b'\1')
                    initial = receive(24)
                    receive(struct.unpack('>I', initial[20:24])[0])

                    def key(code, down):
                        connection.sendall(struct.pack('>BBHI', 4, down, 0, code))
                        time.sleep(0.025)

                    def press(code):
                        key(code, 1)
                        key(code, 0)

                    def type_text(text):
                        for character in text:
                            press(ord(character))

                    def combo(modifier, code):
                        key(modifier, 1)
                        press(code)
                        key(modifier, 0)

                    html = '<title>noVNC isolated test</title><input id="text" autofocus><input id="file" type="file">'
                    target = cdp('Target.createTarget', {'url': 'data:text/html,' + urllib.parse.quote(html)})['targetId']
                    session = cdp('Target.attachToTarget', {'targetId': target, 'flatten': True})['sessionId']
                    cdp('Target.activateTarget', {'targetId': target})
                    time.sleep(1)

                    def evaluate(expression):
                        result = cdp('Runtime.evaluate', {
                            'expression': expression, 'returnByValue': True, 'userGesture': True,
                        }, session)
                        assert 'exceptionDetails' not in result, result
                        return result['result'].get('value')

                    def focus():
                        evaluate("document.getElementById('text').value=''; document.getElementById('text').focus()")
                        time.sleep(0.2)

                    def expect(expected):
                        time.sleep(0.5)
                        actual = evaluate("document.getElementById('text').value")
                        assert actual == expected, (expected, actual)

                    focus()
                    type_text('ai')
                    expect('ai')
                    focus()
                    key(0xffe1, 1)
                    type_text('AI')
                    key(0xffe1, 0)
                    expect('AI')
                    focus()
                    press(0xffe5)
                    type_text('AI')
                    expect('AI')
                    press(0xffe5)
                    focus()
                    type_text('ai')
                    expect('ai')
                    print('PASS 英文、Shift 与 Caps Lock', flush=True)
                    focus()
                    combo(0xffe3, ord(' '))
                    data_home = environment.get('XDG_DATA_HOME', str(Path(environment['HOME']) / '.local/share'))
                    schema = Path(data_home) / 'fcitx5/rime/build/luna_pinyin_simp.schema.yaml'
                    deadline = time.monotonic() + 30
                    while not schema.exists() and time.monotonic() < deadline:
                        time.sleep(0.2)
                    assert schema.exists(), 'Rime 简体拼音方案未部署'
                    time.sleep(0.5)
                    type_text('nihao')
                    press(ord(' '))
                    expect('你好')
                    combo(0xffe3, ord(' '))
                    focus()
                    type_text('ai')
                    expect('ai')
                    print('PASS Rime 拼音与 Ctrl+Space 双向切换', flush=True)
                    evaluate("document.getElementById('file').click()")
                    time.sleep(1)
                    combo(0xffe3, ord('l'))
                    type_text('/app/images/newest.txt')
                    time.sleep(0.5)
                    tree = subprocess.check_output(['xwininfo', '-root', '-tree'], env=environment, text=True)
                    window_id = re.search(r'(0x[0-9a-f]+) "Open File"', tree).group(1)
                    geometry = subprocess.check_output(['xwininfo', '-id', window_id], env=environment, text=True)

                    def coordinate(name):
                        return int(re.search(re.escape(name) + r':\s+(-?\d+)', geometry).group(1))

                    x = coordinate('Absolute upper-left X') + coordinate('Width') - 50
                    y = coordinate('Absolute upper-left Y') + coordinate('Height') - 25
                    for buttons in (0, 1, 0):
                        connection.sendall(struct.pack('>BBHH', 5, buttons, x, y))
                        time.sleep(0.1)
                    time.sleep(1)
                    assert evaluate("document.getElementById('file').files[0]?.name") == 'newest.txt'
                    print('PASS 系统文件窗口选取宿主新增素材', flush=True)
                finally:
                    process.terminate()
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait(timeout=10)
                    os.close(writer)
                    os.close(reader)


if __name__ == '__main__':
    main()
