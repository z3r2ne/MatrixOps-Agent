#!/usr/bin/env python3
"""汇总多轮 explore trace，输出对比统计。"""

from __future__ import annotations

import argparse
import json
import statistics
from pathlib import Path
from typing import Any


def load_summaries(paths: list[Path]) -> list[dict[str, Any]]:
    out = []
    for path in paths:
        data = json.loads(path.read_text(encoding="utf-8"))
        summary = dict(data.get("summary") or {})
        summary["_file"] = path.name
        summary["_status"] = data.get("status", "ok")
        summary["_task_id"] = data.get("task_id")
        out.append(summary)
    return out


def metric(summary: dict[str, Any], key: str, default: int = 0) -> int:
    value = summary.get(key, default)
    return int(value) if value is not None else default


def tool_count(summary: dict[str, Any], *names: str) -> int:
    counts = summary.get("tool_counts") or {}
    total = 0
    for name in names:
        total += int(counts.get(name, 0) or 0)
    return total


def stats(values: list[int]) -> dict[str, float | int]:
    if not values:
        return {"n": 0, "min": 0, "max": 0, "mean": 0, "median": 0, "stdev": 0}
    return {
        "n": len(values),
        "min": min(values),
        "max": max(values),
        "mean": round(statistics.mean(values), 1),
        "median": round(statistics.median(values), 1),
        "stdev": round(statistics.pstdev(values), 1) if len(values) > 1 else 0,
    }


def summarize_project(label: str, summaries: list[dict[str, Any]]) -> dict[str, Any]:
    total_tools = [metric(s, "total_tool_calls") for s in summaries]
    reads = [metric(s, "read_total") for s in summaries]
    greps = [tool_count(s, "rg", "Grep") for s in summaries]
    globs = [tool_count(s, "glob", "Glob") for s in summaries]
    dup_ranges = [len(s.get("read_duplicate_ranges") or []) for s in summaries]
    read_chars = [metric(s, "total_read_output_chars") for s in summaries]
    unique_paths = [metric(s, "read_unique_paths") for s in summaries]

    max_single_file_reads = []
    for s in summaries:
        top = s.get("read_top_paths") or []
        if not top:
            max_single_file_reads.append(0)
            continue
        first = top[0]
        if isinstance(first, (list, tuple)) and len(first) >= 2:
            max_single_file_reads.append(int(first[1]))
        elif isinstance(first, dict):
            max_single_file_reads.append(int(first.get("count", 0)))
        else:
            max_single_file_reads.append(0)

    return {
        "label": label,
        "runs": len(summaries),
        "files": [s.get("_file") for s in summaries],
        "metrics": {
            "total_tool_calls": stats(total_tools),
            "read_total": stats(reads),
            "grep_total": stats(greps),
            "glob_total": stats(globs),
            "read_duplicate_ranges": stats(dup_ranges),
            "read_output_chars": stats(read_chars),
            "read_unique_paths": stats(unique_paths),
            "max_reads_single_file": stats(max_single_file_reads),
        },
    }


def format_stats_row(name: str, ws: dict[str, Any], km: dict[str, Any]) -> str:
    w = ws["metrics"][name]
    k = km["metrics"][name]
    return (
        f"| {name} | {w['mean']} ± {w['stdev']} ({w['min']}–{w['max']}) | "
        f"{k['mean']} ± {k['stdev']} ({k['min']}–{k['max']}) |"
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="汇总多轮 explore batch 结果")
    parser.add_argument("--matrixops-dir", type=Path, required=True)
    parser.add_argument("--kimi-dir", type=Path, required=True)
    parser.add_argument("--output", "-o", type=Path, required=True)
    args = parser.parse_args()

    mo_files = sorted(args.matrixops_dir.glob("matrixops_trace_*.json"))
    km_files = sorted(args.kimi_dir.glob("kimi_trace_*.json"))
    if not mo_files:
        raise SystemExit(f"未找到 matrixops trace: {args.matrixops_dir}")
    if not km_files:
        raise SystemExit(f"未找到 kimi trace: {args.kimi_dir}")

    mo = summarize_project("matrixops", load_summaries(mo_files))
    km = summarize_project("kimi-cli", load_summaries(km_files))

    report = {
        "matrixops": mo,
        "kimi": km,
        "per_run": {
            "matrixops": mo["files"],
            "kimi": km["files"],
        },
    }

    lines = [
        "# Explore 10 轮批量对比",
        "",
        f"- matrixops: {mo['runs']} 轮",
        f"- kimi-cli: {km['runs']} 轮",
        "",
        "| 指标 | matrixops (mean ± σ, min–max) | kimi-cli (mean ± σ, min–max) |",
        "|------|----------------------------------|-------------------------------|",
        format_stats_row("total_tool_calls", mo, km),
        format_stats_row("read_total", mo, km),
        format_stats_row("grep_total", mo, km),
        format_stats_row("glob_total", mo, km),
        format_stats_row("read_duplicate_ranges", mo, km),
        format_stats_row("read_unique_paths", mo, km),
        format_stats_row("max_reads_single_file", mo, km),
        format_stats_row("read_output_chars", mo, km),
        "",
        "## 逐轮明细",
        "",
        "### matrixops",
        "",
        "| run | tools | read | rg | dup_ranges | max_file_reads |",
        "|-----|-------|------|----|-----------:|---------------:|",
    ]

    for i, s in enumerate(load_summaries(mo_files), 1):
        top = s.get("read_top_paths") or []
        max_one = 0
        if top:
            item = top[0]
            max_one = item[1] if isinstance(item, (list, tuple)) else item.get("count", 0)
        lines.append(
            f"| {i} | {metric(s,'total_tool_calls')} | {metric(s,'read_total')} | "
            f"{tool_count(s,'rg')} | {len(s.get('read_duplicate_ranges') or [])} | {max_one} |"
        )

    lines.extend(["", "### kimi-cli", "", "| run | tools | read | grep | dup_ranges | max_file_reads |", "|-----|-------|------|------|-----------:|---------------:|"])
    for i, s in enumerate(load_summaries(km_files), 1):
        top = s.get("read_top_paths") or []
        max_one = 0
        if top:
            item = top[0]
            max_one = item[1] if isinstance(item, (list, tuple)) else item.get("count", 0)
        lines.append(
            f"| {i} | {metric(s,'total_tool_calls')} | {metric(s,'read_total')} | "
            f"{tool_count(s,'Grep')} | {len(s.get('read_duplicate_ranges') or [])} | {max_one} |"
        )

    md = "\n".join(lines) + "\n"
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(md, encoding="utf-8")
    json_path = args.output.with_suffix(".json")
    json_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(md)
    print(f"\n已写入 {args.output}")
    print(f"已写入 {json_path}")


if __name__ == "__main__":
    main()
