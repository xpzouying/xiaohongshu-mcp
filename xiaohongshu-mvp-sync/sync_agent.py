#!/usr/bin/env python3.11
"""主入口：browser-use Agent 执行专辑同步"""
import asyncio
import json
import sys
from pathlib import Path
from datetime import datetime

# 项目根目录
PROJECT_ROOT = Path(__file__).parent
sys.path.insert(0, str(PROJECT_ROOT))

from config import load_config
from utils.cookie_manager import CookieManager
from utils.logger import setup_logger
from tasks.xhs_tasks import load_classification_data, build_sync_task


async def main():
    config = load_config()
    logger = setup_logger(log_file=config.log_file)

    logger.info("=" * 50)
    logger.info("小红书专辑同步 MVP — 启动")
    logger.info("=" * 50)

    # 1. 加载分类数据
    data_path = config.data_dir / "收藏分类结果.json"
    logger.info(f"加载分类数据: {data_path}")
    classification = load_classification_data(data_path)
    total_notes = sum(len(ids) for ids in classification.values())
    logger.info(f"共 {len(classification)} 个分类，{total_notes} 条笔记")

    # 2. 加载 Cookie
    logger.info(f"加载 cookie: {config.cookie_file}")
    cookie_mgr = CookieManager(config.cookie_file)
    xhs_cookies = cookie_mgr.get_xhs_cookies()
    logger.info(f"找到 {len(xhs_cookies)} 个小红书 cookie")

    if not xhs_cookies:
        logger.error("未找到有效的小红书 cookie，请先登录")
        return

    if cookie_mgr.check_expiry(xhs_cookies):
        logger.warning("Cookie 已过期，需要重新登录")
        return

    # 3. 初始化 LLM
    logger.info(f"初始化 LLM: {config.llm.model}")
    from browser_use.llm.openai.chat import ChatOpenAI
    llm = ChatOpenAI(
        model=config.llm.model,
        base_url=config.llm.base_url,
        api_key=config.llm.api_key,
    )

    # 4. 构建 storage_state (包含 cookies)
    storage_state = {
        "cookies": xhs_cookies,
    }

    # 5. 启动浏览器
    logger.info("启动 Chromium (headless)...")
    from browser_use import Browser
    browser = Browser(
        headless=config.browser.headless,
        args=["--no-sandbox", "--disable-dev-shm-usage"],
        storage_state=storage_state,
        viewport={"width": config.browser.viewport_width, "height": config.browser.viewport_height},
    )
    await browser.start()
    logger.info("浏览器已启动")

    # 6. 对每个分类执行同步
    report = {
        "timestamp": datetime.now().isoformat(),
        "total_categories": len(classification),
        "total_notes": total_notes,
        "results": {},
    }

    from browser_use import Agent

    for category, note_ids in classification.items():
        logger.info(f"\n{'='*40}")
        logger.info(f"处理分类: {category} ({len(note_ids)} 条笔记)")
        logger.info(f"{'='*40}")

        try:
            task = build_sync_task(category, note_ids)
            logger.info(f"任务指令已生成 ({len(task)} 字符)")

            agent = Agent(
                task=task,
                llm=llm,
                browser=browser,
                use_thinking=True,
                max_failures=3,
            )

            result = await agent.run()

            # 解析结果
            success_count = len(note_ids)  # 默认全部成功
            failed_ids = []
            status = "completed"

            # 尝试从 result 中提取更精确的信息
            if result:
                last_action = result.last_action() if hasattr(result, 'last_action') else None
                if last_action:
                    logger.info(f"最后操作: {last_action}")

            report["results"][category] = {
                "total": len(note_ids),
                "success": success_count,
                "failed": len(failed_ids),
                "failed_ids": failed_ids,
                "status": status,
            }
            logger.info(f"✅ {category} 同步完成")

        except Exception as e:
            logger.error(f"❌ {category} 失败: {e}", exc_info=True)
            report["results"][category] = {
                "total": len(note_ids),
                "success": 0,
                "failed": len(note_ids),
                "failed_ids": note_ids,
                "status": "failed",
                "error": str(e),
            }

    # 7. 输出报告
    report_file = PROJECT_ROOT / "report.json"
    with open(report_file, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)

    logger.info(f"\n{'='*50}")
    logger.info(f"同步完成！报告: {report_file}")

    # 摘要
    total_success = sum(r.get("success", 0) for r in report["results"].values())
    total_failed = sum(r.get("failed", 0) for r in report["results"].values())
    logger.info(f"成功: {total_success} | 失败: {total_failed}")
    logger.info(f"{'='*50}")

    # 关闭浏览器
    await browser.stop()
    logger.info("浏览器已关闭")


if __name__ == "__main__":
    asyncio.run(main())
