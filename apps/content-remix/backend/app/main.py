from __future__ import annotations

from datetime import datetime, timezone
from typing import Any
from uuid import uuid4

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from .celery_worker import enqueue_job, get_job_result
from .config import get_settings
from .models import (
    CreateRemixJobRequest,
    CreateRemixJobResponse,
    RemixJobListResponse,
    RemixJobStatusResponse,
    RemixJobSummary,
)
from .storage import JobStorage

settings = get_settings()
storage = JobStorage(settings.data_dir)

app = FastAPI(title="ContentRemixAgent API", version="0.1.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


def normalize_status(celery_state: str) -> str:
    mapping = {
        "PENDING": "queued",
        "RECEIVED": "queued",
        "STARTED": "running",
        "RETRY": "running",
        "SUCCESS": "succeeded",
        "FAILURE": "failed",
        "REVOKED": "failed",
    }
    return mapping.get(celery_state.upper(), "queued")


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/api/remix/jobs", response_model=CreateRemixJobResponse)
def create_job(request: CreateRemixJobRequest) -> CreateRemixJobResponse:
    job_id = str(uuid4())
    requested_limit = request.candidate_limit or settings.candidate_limit_default
    candidate_limit = max(1, min(requested_limit, settings.candidate_limit_max))
    payload = {
        "job_id": job_id,
        "keywords": [item.strip() for item in request.keywords if item.strip()],
        "manual_candidates": [item.model_dump() for item in request.manual_candidates],
        "candidate_limit": candidate_limit,
    }
    enqueue_job(payload, task_id=job_id)
    return CreateRemixJobResponse(
        job_id=job_id,
        status="queued",
        created_at=datetime.now(timezone.utc),
    )


@app.get("/api/remix/jobs/{job_id}", response_model=RemixJobStatusResponse)
def job_status(job_id: str) -> RemixJobStatusResponse:
    async_result = get_job_result(job_id)
    detail: Any = async_result.info if isinstance(async_result.info, (dict, list, str)) else None
    return RemixJobStatusResponse(
        job_id=job_id,
        status=normalize_status(async_result.state),
        detail=detail,
    )


@app.get("/api/remix/jobs/{job_id}/result")
def job_result(job_id: str) -> dict[str, Any]:
    payload = storage.load_result(job_id)
    if payload is None:
        raise HTTPException(status_code=404, detail=f"job result not found: {job_id}")
    return payload


@app.get("/api/remix/jobs", response_model=RemixJobListResponse)
def list_jobs(limit: int = 50) -> RemixJobListResponse:
    rows = storage.list_recent(limit=max(1, min(limit, 200)))
    return RemixJobListResponse(
        jobs=[
            RemixJobSummary(
                job_id=row["job_id"],
                updated_at=datetime.fromisoformat(row["updated_at"]),
                has_result=row["has_result"],
            )
            for row in rows
        ]
    )

