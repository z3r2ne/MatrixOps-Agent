#!/usr/bin/env bash
# 续跑未完成的 batch（跳过已有 trace 的轮次）。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KIMI_ROOT="${KIMI_ROOT:-/Users/patrick/Code/kimi-cli}"
BATCH_ID="${BATCH_ID:-20260608_232759}"
RUNS="${RUNS:-10}"
WORK_DIR="${WORK_DIR:-$ROOT}"

MO_OUT="$SCRIPT_DIR/output/batch_${BATCH_ID}/matrixops"
KM_OUT="$KIMI_ROOT/tests/explore_comparison/output/batch_${BATCH_ID}/kimi"
REPORT="$SCRIPT_DIR/output/batch_${BATCH_ID}/report.md"

mkdir -p "$MO_OUT" "$KM_OUT"
[[ -x "$ROOT/explore-compare" ]] || (cd "$ROOT" && go build -o explore-compare ./cmd/explore-compare)

echo "resume batch_id=$BATCH_ID"

for i in $(seq 1 "$RUNS"); do
  stamp="${BATCH_ID}_run$(printf '%02d' "$i")"
  trace="$MO_OUT/matrixops_trace_${stamp}.json"
  if [[ -f "$trace" ]]; then
    echo "[matrixops $i/$RUNS] skip (exists)"
    continue
  fi
  echo ">>> [matrixops $i/$RUNS] $stamp"
  set +e
  WORK_DIR="$WORK_DIR" WORKSPACE_ID="${WORKSPACE_ID:-7}" PROJECT_ID="${PROJECT_ID:-8}" \
  PROMPT_FILE="$SCRIPT_DIR/prompt.txt" OUTPUT="$trace" \
  "$ROOT/explore-compare" 2>&1 | tee "$MO_OUT/matrixops_run_${stamp}.log"
  set -e
done

for i in $(seq 1 "$RUNS"); do
  stamp="${BATCH_ID}_run$(printf '%02d' "$i")"
  trace="$KM_OUT/kimi_trace_${stamp}.json"
  if [[ -f "$trace" ]]; then
    echo "[kimi $i/$RUNS] skip (exists)"
    continue
  fi
  echo ">>> [kimi $i/$RUNS] $stamp"
  set +e
  export KIMI_SHARE_DIR="${KIMI_SHARE_DIR:-$HOME/.kimi-code}"
  export PYTHONPATH="${PYTHONPATH:-}:$KIMI_ROOT/src:$KIMI_ROOT/tests/explore_comparison"
  python3 "$KIMI_ROOT/tests/explore_comparison/run_explore.py" \
    --work-dir "$WORK_DIR" \
    --prompt-file "$SCRIPT_DIR/prompt.txt" \
    --output "$trace" \
    2>&1 | tee "$KM_OUT/kimi_run_${stamp}.log"
  set -e
done

python3 "$SCRIPT_DIR/aggregate_batch.py" \
  --matrixops-dir "$MO_OUT" \
  --kimi-dir "$KM_OUT" \
  --output "$REPORT"

echo "report=$REPORT"
