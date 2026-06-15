#!/usr/bin/env bash
# 对 MatrixOps 与 kimi-cli 各运行 N 轮 explore 对比测试。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KIMI_ROOT="${KIMI_ROOT:-/Users/patrick/Code/kimi-cli}"
RUNS="${RUNS:-10}"
BATCH_ID="${BATCH_ID:-$(date +%Y%m%d_%H%M%S)}"
WORK_DIR="${WORK_DIR:-$ROOT}"

MO_OUT="$SCRIPT_DIR/output/batch_${BATCH_ID}/matrixops"
KM_OUT="$KIMI_ROOT/tests/explore_comparison/output/batch_${BATCH_ID}/kimi"
REPORT="$SCRIPT_DIR/output/batch_${BATCH_ID}/report.md"

mkdir -p "$MO_OUT" "$KM_OUT"

echo "=== Explore batch: $RUNS runs × 2 projects ==="
echo "batch_id=$BATCH_ID"
echo "matrixops_out=$MO_OUT"
echo "kimi_out=$KM_OUT"

if [[ ! -x "$ROOT/explore-compare" ]]; then
  (cd "$ROOT" && go build -o explore-compare ./cmd/explore-compare)
fi

for i in $(seq 1 "$RUNS"); do
  stamp="${BATCH_ID}_run$(printf '%02d' "$i")"
  echo ""
  echo ">>> [matrixops $i/$RUNS] $stamp"
  set +e
  WORK_DIR="$WORK_DIR" \
  WORKSPACE_ID="${WORKSPACE_ID:-7}" \
  PROJECT_ID="${PROJECT_ID:-8}" \
  PROMPT_FILE="$SCRIPT_DIR/prompt.txt" \
  OUTPUT="$MO_OUT/matrixops_trace_${stamp}.json" \
  "$ROOT/explore-compare" 2>&1 | tee "$MO_OUT/matrixops_run_${stamp}.log"
  mo_status=$?
  set -e
  if [[ $mo_status -ne 0 ]]; then
    echo "matrixops run $i failed (exit $mo_status)" >&2
  fi
done

for i in $(seq 1 "$RUNS"); do
  stamp="${BATCH_ID}_run$(printf '%02d' "$i")"
  echo ""
  echo ">>> [kimi $i/$RUNS] $stamp"
  set +e
  export KIMI_SHARE_DIR="${KIMI_SHARE_DIR:-$HOME/.kimi-code}"
  export PYTHONPATH="${PYTHONPATH:-}:$KIMI_ROOT/src:$KIMI_ROOT/tests/explore_comparison"
  python3 "$KIMI_ROOT/tests/explore_comparison/run_explore.py" \
    --work-dir "$WORK_DIR" \
    --prompt-file "$SCRIPT_DIR/prompt.txt" \
    --output "$KM_OUT/kimi_trace_${stamp}.json" \
    2>&1 | tee "$KM_OUT/kimi_run_${stamp}.log"
  km_status=$?
  set -e
  if [[ $km_status -ne 0 ]]; then
    echo "kimi run $i failed (exit $km_status)" >&2
  fi
done

python3 "$SCRIPT_DIR/aggregate_batch.py" \
  --matrixops-dir "$MO_OUT" \
  --kimi-dir "$KM_OUT" \
  --output "$REPORT"

echo ""
echo "report=$REPORT"
