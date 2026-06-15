#!/usr/bin/env python3
"""对比 MatrixOps 与 kimi-cli 的 explore trace，输出差异报告。"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


def load_trace(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def metric_line(label: str, left: Any, right: Any) -> str:
    return f"  {label:28} {str(left):>12}  |  {str(right):>12}"


def compare(matrixops: dict[str, Any], kimi: dict[str, Any]) -> str:
    mo = matrixops.get("summary", {})
    km = kimi.get("summary", {})

    lines = [
        "Explore 对比报告",
        "=" * 72,
        f"{'指标':28} {'matrixops':>12}  |  {'kimi-cli':>12}",
        "-" * 72,
        metric_line("总工具调用", mo.get("total_tool_calls"), km.get("total_tool_calls")),
        metric_line("read 次数", mo.get("read_total"), km.get("read_total")),
        metric_line("rg/grep 次数", mo.get("tool_counts", {}).get("rg") or mo.get("tool_counts", {}).get("Grep", 0),
                    km.get("tool_counts", {}).get("Grep") or km.get("tool_counts", {}).get("rg", 0)),
        metric_line("glob 次数", mo.get("tool_counts", {}).get("glob"), km.get("tool_counts", {}).get("Glob")),
        metric_line("read 涉及文件数", mo.get("read_unique_paths"), km.get("read_unique_paths")),
        metric_line("read 输出总字符", mo.get("total_read_output_chars"), km.get("total_read_output_chars")),
        metric_line("重复 read 区间数", len(mo.get("read_duplicate_ranges", [])), len(km.get("read_duplicate_ranges", []))),
        "",
        "matrixops read TOP 路径:",
    ]
    for path, count in mo.get("read_top_paths", [])[:8]:
        lines.append(f"  {count:4}x  {path}")
    lines.append("")
    lines.append("kimi-cli read TOP 路径:")
    for path, count in km.get("read_top_paths", [])[:8]:
        lines.append(f"  {count:4}x  {path}")

    mo_dup = mo.get("read_duplicate_ranges", [])
    km_dup = km.get("read_duplicate_ranges", [])
    if mo_dup or km_dup:
        lines.extend(["", "重复 read 区间（同 path+offset+limit）:"])
        if mo_dup:
            lines.append("  [matrixops]")
            for item in mo_dup[:10]:
                lines.append(
                    f"    {item.get('count')}x offset={item.get('offset')} limit={item.get('limit')} {item.get('path')}"
                )
        if km_dup:
            lines.append("  [kimi-cli]")
            for item in km_dup[:10]:
                lines.append(
                    f"    {item.get('count')}x offset={item.get('offset')} limit={item.get('limit')} {item.get('path')}"
                )

    lines.extend(["", "行为差异提示:"])
    mo_reads = mo.get("read_total", 0) or 0
    km_reads = km.get("read_total", 0) or 0
    if mo_reads > km_reads * 2 and mo_reads > 20:
        lines.append("  - matrixops read 次数明显偏多，可能存在重复分页读取同一文件")
    if len(mo_dup) > len(km_dup) + 3:
        lines.append("  - matrixops 有更多重复 read 区间，检查记忆压缩是否在工具阶段触发")
    if mo.get("total_tool_calls", 0) > km.get("total_tool_calls", 0) * 1.5:
        lines.append("  - matrixops 总工具步数更多，检查 max_steps 与 explore 搜索策略")

    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(description="对比 explore trace JSON")
    parser.add_argument("matrixops_trace", type=Path, help="MatrixOps trace JSON")
    parser.add_argument("kimi_trace", type=Path, help="kimi-cli trace JSON")
    parser.add_argument("--output", "-o", type=Path, help="可选：写入报告文件")
    args = parser.parse_args()

    report = compare(load_trace(args.matrixops_trace), load_trace(args.kimi_trace))
    print(report)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report + "\n", encoding="utf-8")
        print(f"\n报告已写入 {args.output}")


if __name__ == "__main__":
    main()
