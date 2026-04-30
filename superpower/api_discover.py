"""
阶段 2：API 发现 - 自动捕获小红书真实 API 端点

原理：
  1. 通过 CDP 启用 Network 监听
  2. 启用 Page.setBypassCSP
  3. 手动在页面上操作（创建专辑、移动笔记等）
  4. 自动记录所有 API 请求的 URL、方法、请求体、响应

使用方式：
    1. 启动 Chrome: google-chrome --remote-debugging-port=9222
    2. 登录小红书
    3. 运行: python3 api_discover.py
    4. 在浏览器上手动操作
    5. Ctrl+C 停止，查看 output/xhs_api_capture.json
"""

import json
import time
import sys
import os
import requests
import websockets.sync.client as ws_client

CDP_HOST = "127.0.0.1"
CDP_PORT = 9222
OUTPUT_DIR = "output"

# 只关心这些域名
TARGET_DOMAINS = [
    "edith.xiaohongshu.com",
    "edith.xiaohongshu.com",
    "www.xiaohongshu.com",
    "customer.xiaohongshu.com",
    "creator.xiaohongshu.com",
]


class APICapture:
    """捕获小红书 API 请求"""

    def __init__(self):
        self.ws = None
        self.msg_id = 0
        self.captured_requests = []
        self.pending_bodies = {}  # request_id -> request data

    def connect(self):
        """连接到小红书标签页"""
        url = f"http://{CDP_HOST}:{CDP_PORT}/json"
        targets = requests.get(url, timeout=5).json()

        tab = None
        for t in targets:
            if t.get("type") == "page" and "xiaohongshu" in t.get("url", ""):
                tab = t
                break

        if not tab:
            pages = [t for t in targets if t.get("type") == "page"]
            if pages:
                tab = pages[0]
            else:
                print("❌ 没有可用的浏览器标签页")
                sys.exit(1)

        ws_url = tab["webSocketDebuggerUrl"]
        print(f"🔌 连接到: {tab.get('url', 'unknown')}")
        self.ws = ws_client.connect(ws_url)

    def _send(self, method, params=None):
        """发送 CDP 命令（同步）"""
        self.msg_id += 1
        msg = {"id": self.msg_id, "method": method}
        if params:
            msg["params"] = params
        self.ws.send(json.dumps(msg))

        while True:
            raw = self.ws.recv()
            data = json.loads(raw)
            if data.get("id") == self.msg_id:
                if "error" in data:
                    print(f"⚠️ CDP 错误 [{method}]: {data['error']}")
                return data.get("result", {})

    def _is_target(self, url):
        """判断是否目标域名"""
        return any(d in url for d in TARGET_DOMAINS)

    def _handle_event(self, data):
        """处理 CDP 事件"""
        method = data.get("method", "")
        params = data.get("params", {})

        if method == "Network.requestWillBeSent":
            req_id = params.get("requestId", "")
            request = params.get("request", {})
            url = request.get("url", "")

            if not self._is_target(url):
                return

            # 简化 URL（去掉查询参数中的时间戳等）
            self.pending_bodies[req_id] = {
                "url": url,
                "method": request.get("method", "GET"),
                "headers": dict(request.get("headers", {})),
                "post_data": request.get("postData"),
                "timestamp": params.get("timestamp", 0),
                "type": params.get("type", "Other"),
            }

            # 打印实时信息
            print(f"📡 [{request.get('method', '?')}] {url[:80]}")

        elif method == "Network.responseReceived":
            req_id = params.get("requestId", "")
            response = params.get("response", {})

            if req_id in self.pending_bodies:
                self.pending_bodies[req_id]["status"] = response.get("status", 0)
                self.pending_bodies[req_id]["status_text"] = response.get("statusText", "")
                self.pending_bodies[req_id]["response_headers"] = dict(
                    response.get("headers", {})
                )
                mime = response.get("mimeType", "")
                self.pending_bodies[req_id]["mime_type"] = mime

        elif method == "Network.loadingFinished":
            req_id = params.get("requestId", "")
            if req_id not in self.pending_bodies:
                return

            # 获取响应体
            try:
                body_result = self._send("Network.getResponseBody", {
                    "requestId": req_id
                })
                body = body_result.get("body", "")
                base64 = body_result.get("base64Encoded", False)

                entry = self.pending_bodies.pop(req_id)

                # 只保存 JSON 响应
                mime = entry.get("mime_type", "")
                if "json" in mime or entry.get("url", "").endswith(".json"):
                    entry["response_body"] = body
                    entry["base64"] = base64

                # 只保留关键信息，去掉冗余 headers
                entry["headers"] = {
                    k: v for k, v in entry.get("headers", {}).items()
                    if k.lower() in [
                        "content-type", "authorization", "x-t", "x-s",
                        "x-sign", "cookie", "referer", "origin"
                    ]
                }
                entry["response_headers"] = {
                    k: v for k, v in entry.get("response_headers", {}).items()
                    if k.lower() in ["content-type", "x-trace-id", "set-cookie"]
                }

                self.captured_requests.append(entry)

            except Exception as e:
                print(f"⚠️ 获取响应体失败: {e}")
                if req_id in self.pending_bodies:
                    del self.pending_bodies[req_id]

    def run(self):
        """开始捕获"""
        print("\n🎯 开始捕获小红书 API 请求...")
        print("   请在浏览器中操作（创建专辑、移动笔记等）")
        print("   按 Ctrl+C 停止捕获\n")

        # 1. 启用 CSP Bypass
        print("[1] 启用 CSP Bypass...")
        self._send("Page.setBypassCSP", {"enabled": True})

        # 2. 启用 Network
        print("[2] 启用 Network 监听...")
        self._send("Network.enable")

        # 3. 事件循环
        print("[3] 监听中...\n")
        try:
            while True:
                raw = self.ws.recv()
                data = json.loads(raw)
                if "method" in data:  # 事件（非响应）
                    self._handle_event(data)
        except KeyboardInterrupt:
            pass
        except Exception as e:
            print(f"\n⚠️ 连接断开: {e}")

        # 4. 保存结果
        self._save()

    def _save(self):
        """保存捕获结果"""
        os.makedirs(OUTPUT_DIR, exist_ok=True)
        output_file = os.path.join(OUTPUT_DIR, "xhs_api_capture.json")

        summary = {
            "capture_time": time.strftime("%Y-%m-%d %H:%M:%S"),
            "total_requests": len(self.captured_requests),
            "unique_urls": len(set(r["url"] for r in self.captured_requests)),
            "requests": self.captured_requests,
        }

        with open(output_file, "w", encoding="utf-8") as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)

        print(f"\n📁 已保存 {len(self.captured_requests)} 个请求到 {output_file}")

        # 打印 API 清单
        print("\n📋 捕获的 API 端点:")
        urls = set()
        for r in self.captured_requests:
            if r["url"] not in urls:
                urls.add(r["url"])
                print(f"   [{r['method']}] {r['url'][:100]}")


if __name__ == "__main__":
    capture = APICapture()
    capture.connect()
    capture.run()
