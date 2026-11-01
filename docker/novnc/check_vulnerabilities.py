#!/usr/bin/env python3
"""比较同次构建的镜像，拒绝 GUI 新增或升级为高危的漏洞告警。"""

import collections
import json
import os
from pathlib import Path
import sys


LEVELS = {name: rank for rank, name in enumerate(
    ("Unknown", "Negligible", "Low", "Medium", "High", "Critical")
)}


def findings(path):
    result = {}
    for match in json.loads(Path(path).read_text())["matches"]:
        vulnerability, artifact = match["vulnerability"], match["artifact"]
        key = (vulnerability["id"], artifact["name"], artifact["type"])
        rank = LEVELS[vulnerability["severity"]]
        if key not in result or rank > result[key][0]:
            result[key] = (rank, vulnerability["severity"])
    return result


def main(base_path, gui_path):
    base, gui = findings(base_path), findings(gui_path)
    introduced = sorted(
        key for key, (rank, _) in gui.items()
        if rank >= LEVELS["High"] and rank > base.get(key, (-1, ""))[0]
    )
    lines = ["## 镜像漏洞扫描", "",
             "基线告警单独跟进；本检查拒绝 GUI 新增或严重度升高的 High/Critical 告警。", ""]
    for label, results in (("无 GUI 基线", base), ("GUI 镜像", gui)):
        counts = collections.Counter(severity for _, severity in results.values())
        lines.append(f"- {label}：{dict(counts)}")
    lines.append(f"- 新增或升级的 High/Critical：{len(introduced)}")
    for identifier, package, kind in introduced:
        lines.append(f"  - {identifier}: {package} ({kind})")
    report = "\n".join(lines) + "\n"
    print(report)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(report)
    return bool(introduced)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("用法：check_vulnerabilities.py <base.json> <gui.json>")
    sys.exit(main(*sys.argv[1:]))
