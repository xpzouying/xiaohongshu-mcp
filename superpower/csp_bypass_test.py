"""
超级能力验证：CDP Page.setBypassCSP 绕过小红书 CSP

目标：通过 CDP 的 Page.setBypassCSP 命令关闭 CSP 策略，
      从而在浏览器内直接调用小红书 API（fetch/XHR）

使用方式：
    1. 先确保 Chrome 在调试模式运行：
       google-chrome --remote-debugging-port=9222
       或使用 chrome_launcher.py 启动
    
    2. 手动扫码登录小红书
    
    3. 运行本脚本：
       python3 csp_bypass_test.py
"""

import json
import time
import sys
import requests
import websockets.sync.client as ws_client

CDP_HOST = "127.0.0.1"
CDP_PORT = 9222


def get_targets():
    """获取 Chrome 所有标签页"""
    url = f"http://{CDP_HOST}:{CDP_PORT}/json"
    resp = requests.get(url, timeout=5)
    resp.raise_for_status()
    return resp.json()


def find_xhs_tab():
    """找到小红书相关的标签页"""
    targets = get_targets()
    for t in targets:
        if t.get("type") == "page" and "xiaohongshu" in t.get("url", ""):
            return t
    # 如果没有小红书页面，返回第一个
    pages = [t for t in targets if t.get("type") == "page"]
    if pages:
        return pages[0]
    return None


def cdp_send(ws, method, params=None):
    """发送 CDP 命令"""
    msg_id = int(time.time() * 1000)
    msg = {"id": msg_id, "method": method}
    if params:
        msg["params"] = params
    ws.send(json.dumps(msg))
    
    while True:
        raw = ws.recv()
        data = json.loads(raw)
        if data.get("id") == msg_id:
            return data


def test_bypass_csp():
    print("=" * 60)
    print("🧪 超级能力验证：CSP Bypass")
    print("=" * 60)
    
    # 1. 找到小红书标签页
    print("\n[1/5] 查找小红书标签页...")
    tab = find_xhs_tab()
    if not tab:
        print("❌ 未找到小红书标签页，请先打开 xiaohongshu.com")
        return False
    print(f"✅ 找到: {tab.get('url', 'unknown')}")
    
    # 2. 连接 WebSocket
    print("\n[2/5] 连接 CDP WebSocket...")
    ws_url = tab.get("webSocketDebuggerUrl", "")
    if not ws_url:
        print("❌ 无法获取 WebSocket URL")
        return False
    ws = ws_client.connect(ws_url)
    print("✅ 已连接")
    
    # 3. 测试当前 CSP 状态
    print("\n[3/5] 测试当前 CSP 限制（应该失败）...")
    result = cdp_send(ws, "Runtime.evaluate", {
        "expression": """
            (async () => {
                try {
                    const resp = await fetch('https://edith.xiaohongshu.com/api/sns/v1/user/selfinfo', {
                        method: 'GET',
                        credentials: 'include'
                    });
                    return JSON.stringify({status: resp.status, ok: resp.ok});
                } catch(e) {
                    return JSON.stringify({error: e.message});
                }
            })()
        """,
        "returnByValue": True,
        "awaitPromise": True,
    })
    
    value = result.get("result", {}).get("value", "")
    print(f"   绕过前结果: {value}")
    csp_blocked = "Failed to fetch" in value or "error" in value.lower()
    
    # 4. 启用 CSP Bypass
    print("\n[4/5] 启用 Page.setBypassCSP...")
    bypass_result = cdp_send(ws, "Page.setBypassCSP", {
        "enabled": True
    })
    print(f"   启用结果: {json.dumps(bypass_result, ensure_ascii=False)}")
    
    if "error" in bypass_result:
        print(f"❌ CSP Bypass 失败: {bypass_result['error']}")
        ws.close()
        return False
    
    print("✅ CSP Bypass 已启用")
    
    # 5. 再次测试 API 调用
    print("\n[5/5] 重新测试 API 调用（应该成功）...")
    time.sleep(1)
    result2 = cdp_send(ws, "Runtime.evaluate", {
        "expression": """
            (async () => {
                try {
                    const resp = await fetch('https://edith.xiaohongshu.com/api/sns/v1/user/selfinfo', {
                        method: 'GET',
                        credentials: 'include'
                    });
                    const data = await resp.json();
                    return JSON.stringify({
                        status: resp.status, 
                        ok: resp.ok,
                        hasData: !!data.data
                    });
                } catch(e) {
                    return JSON.stringify({error: e.message});
                }
            })()
        """,
        "returnByValue": True,
        "awaitPromise": True,
    })
    
    value2 = result2.get("result", {}).get("value", "")
    print(f"   绕过结果: {value2}")
    success = "Failed to fetch" not in value2 and "error" not in value2.lower()
    
    ws.close()
    
    print("\n" + "=" * 60)
    if success:
        print("🎉 验证成功！CSP Bypass 生效，API 调用已打通！")
        print("   这意味着：可以通过 CDP 绕过 CSP，直接在浏览器内调用 API")
        print("   技术路线从「UI 自动化」可以升级为「API 直调」")
    else:
        print("⚠️ 验证未完全通过")
        print(f"   绕过前: {value}")
        print(f"   绕过后: {value2}")
        print("   可能需要进一步调整策略")
    print("=" * 60)
    
    return success


if __name__ == "__main__":
    try:
        test_bypass_csp()
    except KeyboardInterrupt:
        print("\n⚠️ 用户中断")
    except Exception as e:
        print(f"\n❌ 异常: {e}")
        import traceback
        traceback.print_exc()
