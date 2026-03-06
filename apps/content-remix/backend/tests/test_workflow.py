from __future__ import annotations

from typing import Any

from app.config import Settings
from app.llm import OpenAICompatibleLLM
from app.storage import JobStorage
from app.workflow import WorkflowDependencies, run_content_remix_job


class FakeMCPClient:
    def __init__(self) -> None:
        self.calls: list[tuple[str, dict[str, Any]]] = []

    def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        self.calls.append((name, arguments))
        if name == "search_feeds":
            return {
                "feeds": [
                    {
                        "id": "feed-from-keyword",
                        "xsecToken": "token-1",
                        "noteCard": {"displayTitle": "爆款标题", "user": {"nickname": "作者A"}},
                    }
                ]
            }
        if name == "get_feed_detail":
            return {
                "feed_id": arguments["feed_id"],
                "data": {
                    "note": {
                        "title": f"title-{arguments['feed_id']}",
                        "desc": "内容描述",
                        "interactInfo": {
                            "likedCount": "123",
                            "commentCount": "12",
                            "collectedCount": "45",
                        },
                    }
                },
            }
        if name == "transcribe_feed_video":
            if arguments["feed_id"] == "manual-fail":
                raise RuntimeError("mock transcribe failed")
            return {
                "feed_id": arguments["feed_id"],
                "transcript_text": "这是转写文本第一句。\n这是第二句。",
                "txt_path": f"/tmp/{arguments['feed_id']}.txt",
                "srt_path": f"/tmp/{arguments['feed_id']}.srt",
            }
        raise AssertionError(f"unexpected tool call: {name}")


def test_run_content_remix_job_merges_candidates_and_persists_result(tmp_path):
    fake_client = FakeMCPClient()
    deps = WorkflowDependencies(
        settings=Settings(
            data_dir=str(tmp_path),
            candidate_limit_default=10,
            candidate_limit_max=20,
            max_parallel_calls=1,
        ),
        storage=JobStorage(str(tmp_path)),
        llm=OpenAICompatibleLLM(base_url=None, api_key=None, model=None),
        mcp_client_factory=lambda: fake_client,
    )
    payload = {
        "job_id": "job-1",
        "keywords": ["穿搭"],
        "manual_candidates": [
            {"feed_id": "manual-1", "xsec_token": "manual-token"},
            {"feed_id": "feed-from-keyword", "xsec_token": "token-1"},
        ],
        "candidate_limit": 2,
    }

    result = run_content_remix_job(payload, deps=deps)
    output = result["output"]
    feed_ids = [item["feed_id"] for item in output["candidates"]]

    assert len(feed_ids) == 2
    assert "manual-1" in feed_ids
    assert "feed-from-keyword" in feed_ids
    assert len(output["remix_ideas"]) >= 1
    assert (tmp_path / "job-1.json").exists()


def test_run_content_remix_job_keeps_running_when_one_transcribe_fails(tmp_path):
    fake_client = FakeMCPClient()
    deps = WorkflowDependencies(
        settings=Settings(
            data_dir=str(tmp_path),
            candidate_limit_default=10,
            candidate_limit_max=20,
            max_parallel_calls=1,
        ),
        storage=JobStorage(str(tmp_path)),
        llm=OpenAICompatibleLLM(base_url=None, api_key=None, model=None),
        mcp_client_factory=lambda: fake_client,
    )
    payload = {
        "job_id": "job-2",
        "keywords": [],
        "manual_candidates": [
            {"feed_id": "manual-fail", "xsec_token": "token-fail"},
            {"feed_id": "manual-ok", "xsec_token": "token-ok"},
        ],
        "candidate_limit": 5,
    }

    result = run_content_remix_job(payload, deps=deps)
    output = result["output"]

    assert any("manual-fail" in item for item in output["errors"])
    assert any(idea["feed_id"] == "manual-ok" for idea in output["remix_ideas"])

