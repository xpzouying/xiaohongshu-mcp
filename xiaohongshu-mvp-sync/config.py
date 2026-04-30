"""配置管理：LLM、浏览器、超时、路径"""
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class LLMConfig:
    model: str = "qwen3.6-plus"
    base_url: str = "https://coding.dashscope.aliyuncs.com/v1"
    api_key: str = ""  # 从环境变量 DASHSCOPE_API_KEY 读取

    def __post_init__(self):
        self.api_key = self.api_key or os.environ.get("DASHSCOPE_API_KEY", "")
        if not self.api_key:
            raise ValueError("DASHSCOPE_API_KEY 环境变量未设置")


@dataclass
class BrowserConfig:
    headless: bool = True
    chrome_path: str = "/usr/bin/chromium-browser"
    viewport_width: int = 1280
    viewport_height: int = 720
    timeout_ms: int = 30000


@dataclass
class ProjectConfig:
    llm: LLMConfig = field(default_factory=LLMConfig)
    browser: BrowserConfig = field(default_factory=BrowserConfig)
    project_root: Path = Path(__file__).parent
    data_dir: Path = field(default_factory=lambda: Path(__file__).parent / "data")
    cookie_file: Path = field(default_factory=lambda: Path(__file__).parent.parent / "cookies.json")
    log_file: Path = field(default_factory=lambda: Path(__file__).parent / "logs" / "sync.log")

    def __post_init__(self):
        self.data_dir.mkdir(exist_ok=True)
        self.log_file.parent.mkdir(exist_ok=True)


def load_config() -> ProjectConfig:
    """加载配置"""
    return ProjectConfig()
