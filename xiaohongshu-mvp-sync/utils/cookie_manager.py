"""Cookie 管理：加载、注入、过期检查"""
import json
import time
from pathlib import Path
from typing import List, Optional


class CookieManager:
    def __init__(self, cookie_path: Path):
        self.cookie_path = cookie_path
        self._cookies = []

    def load_cookies(self) -> List[dict]:
        """从 cookies.json 加载"""
        if not self.cookie_path.exists():
            raise FileNotFoundError(f"Cookie 文件不存在: {self.cookie_path}")
        with open(self.cookie_path, "r") as f:
            raw = json.load(f)
        # 兼容不同格式
        if isinstance(raw, list):
            self._cookies = raw
        elif isinstance(raw, dict):
            self._cookies = raw.get("cookies", [])
        return self._cookies

    def check_expiry(self, cookies: Optional[List[dict]] = None) -> bool:
        """检查 cookie 是否过期，返回 True=已过期"""
        cookies = cookies or self._cookies
        now = time.time()
        for c in cookies:
            expiry = c.get("expiry", c.get("expirationDate"))
            if expiry and now > expiry:
                return True
        return len(cookies) == 0

    def get_xhs_cookies(self) -> List[dict]:
        """过滤出小红书域名的 cookie"""
        cookies = self.load_cookies()
        return [
            c for c in cookies
            if ".xiaohongshu.com" in c.get("domain", "")
            or "xiaohongshu.com" in c.get("domain", "")
        ]
