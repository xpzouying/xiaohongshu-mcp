from __future__ import annotations

from typing import Any

from celery import Celery
from celery.result import AsyncResult

from .config import get_settings
from .workflow import run_content_remix_job

settings = get_settings()

celery_app = Celery(
    "content_remix_agent",
    broker=settings.redis_url,
    backend=settings.redis_url,
)
celery_app.conf.task_track_started = True
celery_app.conf.task_serializer = "json"
celery_app.conf.result_serializer = "json"
celery_app.conf.accept_content = ["json"]


@celery_app.task(name="content_remix.run_job")
def run_content_remix_task(payload: dict[str, Any]) -> dict[str, Any]:
    return run_content_remix_job(payload)


def enqueue_job(payload: dict[str, Any], task_id: str):
    return run_content_remix_task.apply_async(kwargs={"payload": payload}, task_id=task_id)


def get_job_result(task_id: str) -> AsyncResult:
    return AsyncResult(task_id, app=celery_app)

