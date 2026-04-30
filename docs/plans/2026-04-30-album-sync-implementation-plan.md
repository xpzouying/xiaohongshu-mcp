# Implementation Plan: 专辑同步 MVP — browser-use Agent

**日期**: 2026-04-30  
**分支**: `feature/favorite-management`  
**模式**: Subagent-Driven Development (TDD)

---

## Task 1: 基础项目结构和配置

**时间**: ~5 分钟

创建项目目录结构和配置文件。

```bash
mkdir -p xiaohongshu-mvp-sync/{tasks,utils,data}
```

需要创建：
- `xiaohongshu-mvp-sync/config.py` — 配置类（LLM、浏览器路径、超时、数据路径）
- `xiaohongshu-mvp-sync/.env.example` — 环境变量模板
- `data/` 下软链接或复制 `收藏分类结果.json`

**验收**: `python3 config.py` 能正确加载配置，无报错

---

## Task 2: Cookie 管理器

**时间**: ~5 分钟

复用项目根目录的 `cookies.json`。

需要创建：
- `xiaohongshu-mvp-sync/utils/__init__.py`
- `xiaohongshu-mvp-sync/utils/cookie_manager.py` — Cookie 加载 + 注入逻辑

核心方法：
```python
class CookieManager:
    def load_cookies(self) -> list[dict]:
        """从 cookies.json 加载"""
    
    async def inject_cookies(self, browser):
        """将 cookie 注入到浏览器"""
    
    def check_expiry(self) -> bool:
        """检查 cookie 是否过期"""
```

**验收**: 单元测试能正确加载 cookies.json

---

## Task 3: 日志工具

**时间**: ~3 分钟

需要创建：
- `xiaohongshu-mvp-sync/utils/logger.py` — 结构化日志

要求：
- 输出到控制台 + 日志文件
- 按分类记录操作（"创建专辑: 技术 ✅"）
- 记录失败信息

**验收**: 日志输出正常

---

## Task 4: 同步任务定义

**时间**: ~5 分钟

定义每个分类的同步任务。

需要创建：
- `xiaohongshu-mvp-sync/tasks/__init__.py`
- `xiaohongshu-mvp-sync/tasks/xhs_tasks.py`

核心逻辑：
```python
def build_sync_task(album_name: str, note_ids: list[str]) -> str:
    """根据专辑名和笔记ID列表生成 Agent 任务指令"""
    
def load_classification_data(path: str) -> dict:
    """读取 收藏分类结果.json，返回 {分类名: [note_ids]}"""
```

**验收**: 能正确解析分类数据并生成任务指令

---

## Task 5: 主 Agent 脚本

**时间**: ~10 分钟

核心执行逻辑。

需要创建：
- `xiaohongshu-mvp-sync/sync_agent.py`

核心流程：
```python
async def main():
    # 1. 初始化配置
    config = load_config()
    
    # 2. 启动浏览器
    browser = Browser(headless=True)
    
    # 3. 注入 cookie
    cookie_mgr = CookieManager()
    await cookie_mgr.inject_cookies(browser)
    
    # 4. 加载分类数据
    data = load_classification_data()
    
    # 5. 对每个分类创建 Agent 执行同步
    for category, note_ids in data.items():
        agent = Agent(
            task=build_sync_task(category, note_ids),
            llm=config.llm,
            browser=browser,
        )
        result = await agent.run()
        report.add(category, result)
    
    # 6. 输出报告
    print_report(report)
```

**验收**: 
- 脚本能运行不报错
- 浏览器能启动并导航到小红书
- cookie 注入后能确认登录状态

---

## Task 6: 同步报告

**时间**: ~3 分钟

输出同步结果。

需要创建：
- `xiaohongshu-mvp-sync/report.py`

输出格式（JSON + 可读文本）：
```json
{
  "timestamp": "2026-04-30T15:00:00",
  "total_categories": 7,
  "total_notes": 145,
  "results": {
    "技术干货": {"success": 20, "failed": 2, "failed_ids": [...]},
    "生活记录": {"success": 15, "failed": 0, "failed_ids": []}
  }
}
```

**验收**: 报告格式正确，数据完整

---

## Task 7: 端到端测试

**时间**: ~15 分钟

在真实环境跑完整流程。

步骤：
1. 确保 cookies.json 有效
2. 运行 `python3 sync_agent.py`
3. 观察浏览器操作
4. 检查同步报告
5. 手动验证小红书网页版

**验收**: 145 条笔记全部正确同步到对应专辑

---

## 执行顺序

```
Task 1 → Task 2 → Task 3 → Task 4 → Task 5 → Task 6 → Task 7
  (配置)    (Cookie)  (日志)    (任务)     (Agent)   (报告)   (测试)
```

前 6 个任务可以批量完成，Task 7 必须手动验证。

---

*计划完成 → 进入 Phase 3: Subagent-Driven Development*
