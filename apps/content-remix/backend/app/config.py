from __future__ import annotations

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    mcp_url: str = "http://localhost:18060/mcp"
    redis_url: str = "redis://localhost:6379/0"
    data_dir: str = "data/content-remix/jobs"

    candidate_limit_default: int = 10
    candidate_limit_max: int = 20
    max_parallel_calls: int = 3
    request_timeout_seconds: int = 60

    llm_base_url: str | None = None
    llm_api_key: str | None = None
    llm_model: str | None = None
    llm_timeout_seconds: int = 60

    transcribe_provider: str | None = None
    transcribe_model: str | None = None
    transcribe_model_path: str | None = None

    model_config = SettingsConfigDict(
        env_prefix="CONTENT_REMIX_",
        env_file=".env",
        extra="ignore",
    )


def get_settings() -> Settings:
    return Settings()
