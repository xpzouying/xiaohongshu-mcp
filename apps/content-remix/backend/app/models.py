from __future__ import annotations

from datetime import datetime
from typing import Any
from uuid import uuid4

from pydantic import BaseModel, Field, model_validator


class ManualCandidate(BaseModel):
    feed_id: str = Field(min_length=1)
    xsec_token: str = Field(min_length=1)


class CreateRemixJobRequest(BaseModel):
    keywords: list[str] = Field(default_factory=list)
    manual_candidates: list[ManualCandidate] = Field(default_factory=list)
    candidate_limit: int | None = None

    @model_validator(mode="after")
    def validate_input_sources(self) -> "CreateRemixJobRequest":
        has_keywords = any(item.strip() for item in self.keywords)
        has_manual = len(self.manual_candidates) > 0
        if not has_keywords and not has_manual:
            raise ValueError("keywords 和 manual_candidates 不能同时为空")
        return self


class CreateRemixJobResponse(BaseModel):
    job_id: str
    status: str
    created_at: datetime

    @classmethod
    def build(cls, job_id: str | None = None) -> "CreateRemixJobResponse":
        return cls(
            job_id=job_id or str(uuid4()),
            status="queued",
            created_at=datetime.utcnow(),
        )


class RemixJobStatusResponse(BaseModel):
    job_id: str
    status: str
    detail: Any = None


class RemixJobSummary(BaseModel):
    job_id: str
    updated_at: datetime
    has_result: bool


class RemixJobListResponse(BaseModel):
    jobs: list[RemixJobSummary]

