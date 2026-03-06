from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Callable, TypedDict

from langgraph.graph import END, StateGraph

from .config import Settings, get_settings
from .llm import OpenAICompatibleLLM
from .mcp_client import MCPClient
from .storage import JobStorage


class RemixState(TypedDict, total=False):
    job_id: str
    payload: dict[str, Any]
    candidates: list[dict[str, Any]]
    errors: list[str]
    result_path: str
    output: dict[str, Any]


@dataclass
class WorkflowDependencies:
    settings: Settings
    storage: JobStorage
    llm: OpenAICompatibleLLM
    mcp_client_factory: Callable[[], MCPClient]


def default_dependencies(settings: Settings | None = None) -> WorkflowDependencies:
    cfg = settings or get_settings()
    return WorkflowDependencies(
        settings=cfg,
        storage=JobStorage(cfg.data_dir),
        llm=OpenAICompatibleLLM(
            base_url=cfg.llm_base_url,
            api_key=cfg.llm_api_key,
            model=cfg.llm_model,
            timeout_seconds=cfg.llm_timeout_seconds,
        ),
        mcp_client_factory=lambda: MCPClient(cfg.mcp_url, timeout_seconds=cfg.request_timeout_seconds),
    )


def run_content_remix_job(payload: dict[str, Any], deps: WorkflowDependencies | None = None) -> dict[str, Any]:
    dependencies = deps or default_dependencies()
    graph = _build_graph(dependencies)
    initial: RemixState = {
        "job_id": payload["job_id"],
        "payload": payload,
        "errors": [],
        "candidates": [],
    }
    result = graph.invoke(initial)
    return {
        "job_id": payload["job_id"],
        "result_path": result["result_path"],
        "output": result["output"],
        "errors": result.get("errors", []),
    }


def _build_graph(deps: WorkflowDependencies):
    builder = StateGraph(RemixState)
    builder.add_node("collect_candidates", lambda state: _collect_candidates_node(state, deps))
    builder.add_node("fetch_details", lambda state: _fetch_details_node(state, deps))
    builder.add_node("transcribe_videos", lambda state: _transcribe_videos_node(state, deps))
    builder.add_node("analyze_content", lambda state: _analyze_content_node(state, deps))
    builder.add_node("persist_result", lambda state: _persist_result_node(state, deps))

    builder.set_entry_point("collect_candidates")
    builder.add_edge("collect_candidates", "fetch_details")
    builder.add_edge("fetch_details", "transcribe_videos")
    builder.add_edge("transcribe_videos", "analyze_content")
    builder.add_edge("analyze_content", "persist_result")
    builder.add_edge("persist_result", END)

    return builder.compile()


def _collect_candidates_node(state: RemixState, deps: WorkflowDependencies) -> dict[str, Any]:
    payload = state["payload"]
    errors = list(state.get("errors", []))
    candidate_map: dict[str, dict[str, Any]] = {}

    for item in payload.get("manual_candidates", []):
        feed_id = (item.get("feed_id") or "").strip()
        xsec_token = (item.get("xsec_token") or "").strip()
        if not feed_id or not xsec_token:
            continue
        candidate_map[feed_id] = {
            "feed_id": feed_id,
            "xsec_token": xsec_token,
            "source": "manual",
            "keyword": None,
        }

    keywords = [keyword.strip() for keyword in payload.get("keywords", []) if keyword and keyword.strip()]
    if keywords:
        client = deps.mcp_client_factory()
        for keyword in keywords:
            try:
                response = client.call_tool(
                    "search_feeds",
                    {
                        "keyword": keyword,
                        "filters": {
                            "note_type": "视频",
                            "sort_by": "最多点赞",
                        },
                    },
                )
                feeds = response.get("feeds", []) if isinstance(response, dict) else []
                for feed in feeds:
                    feed_id = (feed.get("id") or "").strip()
                    xsec_token = (feed.get("xsecToken") or "").strip()
                    if not feed_id or not xsec_token:
                        continue
                    if feed_id in candidate_map:
                        continue
                    note_card = feed.get("noteCard") or {}
                    user = note_card.get("user") or {}
                    candidate_map[feed_id] = {
                        "feed_id": feed_id,
                        "xsec_token": xsec_token,
                        "source": "keyword",
                        "keyword": keyword,
                        "title": note_card.get("displayTitle"),
                        "author": user.get("nickname") or user.get("nickName"),
                    }
            except Exception as exc:
                errors.append(f"关键词[{keyword}] 搜索失败: {exc}")

    candidate_limit = int(payload.get("candidate_limit") or deps.settings.candidate_limit_default)
    candidate_limit = max(1, min(candidate_limit, deps.settings.candidate_limit_max))
    candidates = list(candidate_map.values())[:candidate_limit]

    return {
        "candidates": candidates,
        "errors": errors,
    }


def _fetch_details_node(state: RemixState, deps: WorkflowDependencies) -> dict[str, Any]:
    candidates = [dict(item) for item in state.get("candidates", [])]
    errors = list(state.get("errors", []))
    if not candidates:
        return {"candidates": candidates, "errors": errors}

    max_workers = max(1, deps.settings.max_parallel_calls)

    def fetch(candidate: dict[str, Any]) -> tuple[str, dict[str, Any] | None, str | None]:
        client = deps.mcp_client_factory()
        try:
            detail = client.call_tool(
                "get_feed_detail",
                {
                    "feed_id": candidate["feed_id"],
                    "xsec_token": candidate["xsec_token"],
                    "load_all_comments": False,
                },
            )
            return candidate["feed_id"], detail if isinstance(detail, dict) else None, None
        except Exception as exc:
            return candidate["feed_id"], None, str(exc)

    detail_by_feed: dict[str, dict[str, Any]] = {}
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = [executor.submit(fetch, candidate) for candidate in candidates]
        for future in as_completed(futures):
            feed_id, detail, error = future.result()
            if error:
                errors.append(f"获取详情失败[{feed_id}]: {error}")
                continue
            if detail:
                detail_by_feed[feed_id] = detail

    for candidate in candidates:
        candidate["detail"] = detail_by_feed.get(candidate["feed_id"])
    return {"candidates": candidates, "errors": errors}


def _transcribe_videos_node(state: RemixState, deps: WorkflowDependencies) -> dict[str, Any]:
    candidates = [dict(item) for item in state.get("candidates", [])]
    errors = list(state.get("errors", []))
    if not candidates:
        return {"candidates": candidates, "errors": errors}

    max_workers = max(1, deps.settings.max_parallel_calls)

    def transcribe(candidate: dict[str, Any]) -> tuple[str, dict[str, Any] | None, str | None]:
        client = deps.mcp_client_factory()
        args: dict[str, Any] = {
            "feed_id": candidate["feed_id"],
            "xsec_token": candidate["xsec_token"],
            "language": "zh",
            "keep_artifacts": False,
        }
        if deps.settings.transcribe_provider:
            args["provider"] = deps.settings.transcribe_provider
        model_name = deps.settings.transcribe_model or deps.settings.transcribe_model_path
        if model_name:
            args["model"] = model_name
        try:
            result = client.call_tool("transcribe_feed_video", args)
            return candidate["feed_id"], result if isinstance(result, dict) else None, None
        except Exception as exc:
            return candidate["feed_id"], None, str(exc)

    transcription_by_feed: dict[str, dict[str, Any]] = {}
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        futures = [executor.submit(transcribe, candidate) for candidate in candidates]
        for future in as_completed(futures):
            feed_id, result, error = future.result()
            if error:
                errors.append(f"转写失败[{feed_id}]: {error}")
                continue
            if result:
                transcription_by_feed[feed_id] = result

    for candidate in candidates:
        candidate["transcription"] = transcription_by_feed.get(candidate["feed_id"])
    return {"candidates": candidates, "errors": errors}


def _analyze_content_node(state: RemixState, deps: WorkflowDependencies) -> dict[str, Any]:
    candidates = [dict(item) for item in state.get("candidates", [])]
    for candidate in candidates:
        detail = _extract_note_detail(candidate.get("detail"))
        transcript = (candidate.get("transcription") or {}).get("transcript_text", "")
        fallback = _fallback_analysis(candidate, detail, transcript)
        llm_output = _llm_analysis(deps.llm, candidate, detail, transcript)
        if llm_output:
            fallback["viral_breakdown"].update(
                {
                    key: value
                    for key, value in llm_output.get("viral_breakdown", {}).items()
                    if value is not None
                }
            )
            if llm_output.get("remix_idea"):
                fallback["remix_idea"].update(
                    {key: value for key, value in llm_output["remix_idea"].items() if value is not None}
                )
        candidate["viral_breakdown"] = fallback["viral_breakdown"]
        candidate["remix_idea"] = fallback["remix_idea"]

    return {"candidates": candidates}


def _persist_result_node(state: RemixState, deps: WorkflowDependencies) -> dict[str, Any]:
    job_id = state["job_id"]
    candidates = state.get("candidates", [])
    errors = state.get("errors", [])
    output = {
        "job_id": job_id,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "candidate_count": len(candidates),
        "candidates": candidates,
        "viral_breakdown": [item.get("viral_breakdown") for item in candidates if item.get("viral_breakdown")],
        "remix_ideas": [item.get("remix_idea") for item in candidates if item.get("remix_idea")],
        "errors": errors,
    }
    path = deps.storage.save_result(job_id, output)
    return {
        "result_path": str(path),
        "output": output,
        "errors": errors,
    }


def _extract_note_detail(detail_payload: Any) -> dict[str, Any]:
    if not isinstance(detail_payload, dict):
        return {}
    data = detail_payload.get("data")
    if not isinstance(data, dict):
        return {}
    note = data.get("note")
    return note if isinstance(note, dict) else {}


def _fallback_analysis(candidate: dict[str, Any], detail: dict[str, Any], transcript: str) -> dict[str, Any]:
    title = (detail.get("title") or candidate.get("title") or "").strip()
    desc = (detail.get("desc") or "").strip()
    interact = detail.get("interactInfo") or {}
    liked_count = interact.get("likedCount", "0")
    comment_count = interact.get("commentCount", "0")
    collected_count = interact.get("collectedCount", "0")

    first_lines = [line.strip() for line in transcript.splitlines() if line.strip()]
    hook_line = first_lines[0] if first_lines else (desc[:40] if desc else title[:40])

    breakdown = {
        "feed_id": candidate["feed_id"],
        "content_structure": "开场钩子 -> 核心信息 -> 互动收尾",
        "topic_strategy": f"围绕「{title or '内容主题'}」提供可执行经验",
        "emotional_value": "通过真实体验降低用户决策焦虑",
        "hook_style": hook_line,
        "engagement_signals": {
            "liked_count": liked_count,
            "comment_count": comment_count,
            "collected_count": collected_count,
        },
        "risks": [
            "避免夸大收益或医疗/金融类违规承诺",
            "避免搬运原视频文案，需加入个人经验",
        ],
    }

    idea = {
        "feed_id": candidate["feed_id"],
        "angle": f"从 {title or '同主题'} 的常见误区切入",
        "title": f"{(title or '这条内容')[:14]}，我会这样二创",
        "opening_hook": f"别急着照搬，这条{title or '内容'}真正打动人的是这一步。",
        "script_outline": [
            "30秒复述原视频核心冲突",
            "补充1个反常识观点和1个执行细节",
            "用真实体验结尾并抛出互动问题",
        ],
        "visual_suggestions": [
            "前3秒给出结果画面",
            "中段加入步骤字幕",
            "结尾展示前后对比截图",
        ],
        "tags": ["#二创灵感", "#小红书选题", "#内容拆解"],
        "risk_notes": ["引用原内容时避免出现未经授权的完整片段"],
    }

    return {"viral_breakdown": breakdown, "remix_idea": idea}


def _llm_analysis(
    llm: OpenAICompatibleLLM,
    candidate: dict[str, Any],
    detail: dict[str, Any],
    transcript: str,
) -> dict[str, Any] | None:
    if not llm.enabled:
        return None

    system_prompt = (
        "你是内容二创分析助手。仅输出 JSON，对象包含 viral_breakdown 和 remix_idea 两个字段。"
    )
    user_prompt = (
        "请基于以下素材做爆款拆解和二创建议。\n"
        f"feed_id: {candidate.get('feed_id')}\n"
        f"title: {detail.get('title') or candidate.get('title')}\n"
        f"desc: {detail.get('desc')}\n"
        f"transcript: {transcript[:1800]}\n"
        "输出字段要求：\n"
        "viral_breakdown: content_structure, topic_strategy, emotional_value, hook_style, engagement_signals, risks\n"
        "remix_idea: angle, title, opening_hook, script_outline, visual_suggestions, tags, risk_notes\n"
    )
    return llm.generate_json(system_prompt, user_prompt)
