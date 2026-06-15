#!/usr/bin/env python3
"""从 MatrixOps SQLite 数据库提取 explore 会话的工具调用 trace。"""

from __future__ import annotations

import argparse
import json
import os
import sqlite3
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


DEFAULT_DB = Path.home() / ".matrixops" / "matrixops.db"


def resolve_db_path(explicit: str | None) -> Path:
    if explicit:
        return Path(explicit).expanduser()
    env = os.environ.get("MATRIXOPS_DB", "").strip()
    if env:
        return Path(env).expanduser()
    return DEFAULT_DB


def load_session_id_by_task(conn: sqlite3.Connection, task_id: int) -> str:
    row = conn.execute("SELECT session_id FROM tasks WHERE id = ?", (task_id,)).fetchone()
    if not row or not row[0]:
        raise SystemExit(f"task {task_id} 不存在或缺少 session_id")
    return str(row[0])


def extract_tool_calls(conn: sqlite3.Connection, session_id: str) -> list[dict[str, Any]]:
    rows = conn.execute(
        """
        SELECT p.tool, p.time_created
        FROM parts p
        WHERE p.session_id = ? AND p.type = 'tool'
        ORDER BY p.time_created
        """,
        (session_id,),
    ).fetchall()

    calls: list[dict[str, Any]] = []
    for tool_json, ts in rows:
        payload = json.loads(tool_json)
        name = payload.get("tool", "")
        state = payload.get("state", {}) or {}
        inp = state.get("input", {}) or {}
        output = state.get("output") or ""
        calls.append(
            {
                "tool": name,
                "status": state.get("status"),
                "input": inp,
                "output_chars": len(output),
                "time_created": ts,
            }
        )
    return calls


def summarize(calls: list[dict[str, Any]]) -> dict[str, Any]:
    tool_counts = Counter(c["tool"] for c in calls)
    read_calls = [c for c in calls if c["tool"] == "read"]
    read_by_path = Counter(c["input"].get("path") for c in read_calls)
    read_keys = Counter(
        (c["input"].get("path"), c["input"].get("offset"), c["input"].get("limit"))
        for c in read_calls
    )
    duplicates = [(k, n) for k, n in read_keys.items() if n > 1]
    duplicates.sort(key=lambda item: item[1], reverse=True)

    return {
        "total_tool_calls": len(calls),
        "tool_counts": dict(tool_counts.most_common()),
        "read_total": len(read_calls),
        "read_unique_paths": len(read_by_path),
        "read_top_paths": read_by_path.most_common(15),
        "read_duplicate_ranges": [
            {"path": k[0], "offset": k[1], "limit": k[2], "count": n}
            for k, n in duplicates[:20]
        ],
        "total_read_output_chars": sum(c["output_chars"] for c in read_calls),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="提取 MatrixOps explore 工具调用 trace")
    parser.add_argument("--session-id", help="会话 ID")
    parser.add_argument("--task-id", type=int, help="任务 ID（自动解析 session_id）")
    parser.add_argument("--db", help="数据库路径，默认 ~/.matrixops/matrixops.db")
    parser.add_argument(
        "--output",
        "-o",
        type=Path,
        default=Path("matrixops_trace.json"),
        help="输出 JSON 路径",
    )
    args = parser.parse_args()

    if not args.session_id and not args.task_id:
        parser.error("需要 --session-id 或 --task-id")

    db_path = resolve_db_path(args.db)
    if not db_path.is_file():
        raise SystemExit(f"数据库不存在: {db_path}")

    conn = sqlite3.connect(db_path)
    try:
        session_id = args.session_id or load_session_id_by_task(conn, args.task_id)
        calls = extract_tool_calls(conn, session_id)
    finally:
        conn.close()

    trace = {
        "project": "matrixops",
        "worker": "explore",
        "session_id": session_id,
        "task_id": args.task_id,
        "db_path": str(db_path),
        "collected_at": datetime.now(timezone.utc).isoformat(),
        "summary": summarize(calls),
        "tool_calls": calls,
    }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(trace, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"已写入 {args.output} ({len(calls)} 次工具调用)")


if __name__ == "__main__":
    main()
