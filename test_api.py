#!/usr/bin/env python3
"""测试 MCP API"""
import requests
import json

BASE = "http://localhost:18060"

print("测试 1: 健康检查")
r = requests.get(f"{BASE}/health", timeout=5)
print(f"  {r.json()}")

print("\n测试 2: 获取专辑列表")
r = requests.get(f"{BASE}/api/v1/albums/list", timeout=30)
print(f"  {r.json()}")

print("\n测试 3: 创建专辑")
r = requests.post(f"{BASE}/api/v1/albums/create", json={"name": "测试专辑 123"}, timeout=120)
print(f"  {r.json()}")

print("\n测试 4: 获取收藏列表")
r = requests.get(f"{BASE}/api/v1/favorites/list", timeout=60)
d = r.json()
print(f"  Success: {d.get('success')}, Count: {d.get('data',{}).get('count',0)}")
