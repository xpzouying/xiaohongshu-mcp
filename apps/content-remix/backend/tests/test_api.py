from __future__ import annotations

from datetime import datetime, timezone

import pytest
from fastapi.testclient import TestClient

from app import main
from app.storage import JobStorage


class FakeAsyncResult:
    def __init__(self, state: str, info=None) -> None:
        self.state = state
        self.info = info


@pytest.fixture
def client(tmp_path, monkeypatch):
    monkeypatch.setattr(main, "storage", JobStorage(str(tmp_path)))
    return TestClient(main.app)


def test_create_job_endpoint_enqueues_task(client, monkeypatch):
    captured = {}

    def fake_enqueue(payload, task_id):
        captured["payload"] = payload
        captured["task_id"] = task_id
        return None

    monkeypatch.setattr(main, "enqueue_job", fake_enqueue)

    response = client.post(
        "/api/remix/jobs",
        json={
            "keywords": ["减脂餐"],
            "manual_candidates": [],
            "candidate_limit": 10,
        },
    )

    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "queued"
    assert body["job_id"] == captured["task_id"]
    assert captured["payload"]["keywords"] == ["减脂餐"]


def test_status_endpoint_maps_celery_state(client, monkeypatch):
    monkeypatch.setattr(main, "get_job_result", lambda _: FakeAsyncResult("STARTED", {"step": "transcribe"}))
    response = client.get("/api/remix/jobs/job-abc")
    assert response.status_code == 200
    assert response.json()["status"] == "running"
    assert response.json()["detail"] == {"step": "transcribe"}


def test_result_endpoint_reads_json_result(client, monkeypatch):
    payload = {
        "job_id": "job-result",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "candidate_count": 0,
        "candidates": [],
        "viral_breakdown": [],
        "remix_ideas": [],
        "errors": [],
    }
    main.storage.save_result("job-result", payload)
    response = client.get("/api/remix/jobs/job-result/result")
    assert response.status_code == 200
    assert response.json()["job_id"] == "job-result"

