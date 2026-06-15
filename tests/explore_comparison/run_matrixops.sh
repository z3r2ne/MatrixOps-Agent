#!/usr/bin/env bash
# 在 MatrixOps 项目根目录运行 explore worker，并收集工具调用 trace。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROMPT_FILE="${PROMPT_FILE:-$SCRIPT_DIR/prompt.txt}"
WORK_DIR="${WORK_DIR:-$ROOT}"
OUTPUT_DIR="${OUTPUT_DIR:-$SCRIPT_DIR/output}"
COMPARE_BIN="${COMPARE_BIN:-$ROOT/explore-compare}"
STAMP="$(date +%Y%m%d_%H%M%S)"
TRACE_FILE="$OUTPUT_DIR/matrixops_trace_${STAMP}.json"
WORKSPACE_ID="${WORKSPACE_ID:-7}"
PROJECT_ID="${PROJECT_ID:-8}"

mkdir -p "$OUTPUT_DIR"

if [[ ! -x "$COMPARE_BIN" ]]; then
  echo "正在构建 explore-compare..."
  (cd "$ROOT" && go build -o "$COMPARE_BIN" ./cmd/explore-compare)
fi

echo "workdir: $WORK_DIR"
echo "trace: $TRACE_FILE"

WORK_DIR="$WORK_DIR" \
WORKSPACE_ID="$WORKSPACE_ID" \
PROJECT_ID="$PROJECT_ID" \
PROMPT_FILE="$PROMPT_FILE" \
OUTPUT="$TRACE_FILE" \
"$COMPARE_BIN" 2>&1 | tee "$OUTPUT_DIR/matrixops_run_${STAMP}.log"

echo "trace=$TRACE_FILE"
