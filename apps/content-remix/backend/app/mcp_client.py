from __future__ import annotations

import json
import itertools
import threading
from typing import Any

import requests


class MCPClientError(RuntimeError):
    """Raised when MCP call fails."""


class MCPClient:
    def __init__(self, server_url: str, timeout_seconds: int = 60) -> None:
        self.server_url = server_url
        self.timeout_seconds = timeout_seconds
        self._session_id: str | None = None
        self._id_gen = itertools.count(1)
        self._lock = threading.Lock()
        self._http = requests.Session()

    def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        self._ensure_initialized()
        response = self._rpc("tools/call", {"name": name, "arguments": arguments})
        if "error" in response:
            raise MCPClientError(f"MCP 调用失败 [{name}]: {response['error']}")
        result = response.get("result", {})
        if result.get("isError"):
            text = self._extract_text(result)
            raise MCPClientError(f"MCP 工具返回错误 [{name}]: {text}")
        return self._parse_content(result)

    def _ensure_initialized(self) -> None:
        with self._lock:
            if self._session_id:
                return

            init_payload = {"jsonrpc": "2.0", "method": "initialize", "params": {}, "id": next(self._id_gen)}
            init_resp = self._http.post(
                self.server_url,
                json=init_payload,
                timeout=self.timeout_seconds,
                headers={"Content-Type": "application/json"},
            )
            init_resp.raise_for_status()
            data = init_resp.json()
            if "error" in data:
                raise MCPClientError(f"MCP initialize 失败: {data['error']}")

            session_id = init_resp.headers.get("Mcp-Session-Id")
            if not session_id:
                raise MCPClientError("MCP initialize 成功但缺少 Mcp-Session-Id")

            self._session_id = session_id
            notify_payload = {"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}}
            notify_resp = self._http.post(
                self.server_url,
                json=notify_payload,
                timeout=self.timeout_seconds,
                headers={
                    "Content-Type": "application/json",
                    "Mcp-Session-Id": self._session_id,
                },
            )
            notify_resp.raise_for_status()

    def _rpc(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": next(self._id_gen),
        }
        headers = {"Content-Type": "application/json"}
        if self._session_id:
            headers["Mcp-Session-Id"] = self._session_id

        response = self._http.post(
            self.server_url,
            json=payload,
            timeout=self.timeout_seconds,
            headers=headers,
        )
        response.raise_for_status()
        return response.json()

    @staticmethod
    def _extract_text(result: dict[str, Any]) -> str:
        content = result.get("content", [])
        text_chunks = [item.get("text", "") for item in content if item.get("type") == "text"]
        return "\n".join(chunk for chunk in text_chunks if chunk).strip()

    def _parse_content(self, result: dict[str, Any]) -> Any:
        text = self._extract_text(result)
        if not text:
            return result
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text

