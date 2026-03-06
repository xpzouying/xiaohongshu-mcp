from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


class JobStorage:
    def __init__(self, base_dir: str) -> None:
        self.base_dir = Path(base_dir)
        self.base_dir.mkdir(parents=True, exist_ok=True)

    def result_path(self, job_id: str) -> Path:
        return self.base_dir / f"{job_id}.json"

    def save_result(self, job_id: str, payload: dict[str, Any]) -> Path:
        path = self.result_path(job_id)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        return path

    def load_result(self, job_id: str) -> dict[str, Any] | None:
        path = self.result_path(job_id)
        if not path.exists():
            return None
        return json.loads(path.read_text(encoding="utf-8"))

    def list_recent(self, limit: int = 50) -> list[dict[str, Any]]:
        rows: list[dict[str, Any]] = []
        for path in sorted(self.base_dir.glob("*.json"), key=lambda p: p.stat().st_mtime, reverse=True):
            mtime = datetime.fromtimestamp(path.stat().st_mtime, tz=timezone.utc).isoformat()
            rows.append(
                {
                    "job_id": path.stem,
                    "updated_at": mtime,
                    "has_result": True,
                }
            )
            if len(rows) >= limit:
                break
        return rows

