#!/usr/bin/env bash
# Run the full Go test matrix for CI and write logs under ci-logs/ for debugging.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${CI_TEST_LOG_DIR:-$ROOT/ci-logs}"
mkdir -p "$LOG_DIR"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_FILE="$LOG_DIR/test-$STAMP.log"
FAILURES_FILE="$LOG_DIR/failures-$STAMP.txt"
SUMMARY_FILE="$LOG_DIR/summary-$STAMP.md"

: >"$FAILURES_FILE"

MODULES=(
  "."
  "agent"
  "agent/core_agent"
  "agent/memory"
  "pkgs"
  "tests"
  "web-server"
)

{
  echo "# MatrixOps CI Test Run"
  echo
  echo "- started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "- commit: ${GITHUB_SHA:-local}"
  echo "- ref: ${GITHUB_REF:-local}"
  echo
} >"$SUMMARY_FILE"

failed=0
failed_modules=()

run_module_tests() {
  local module="$1"
  local slug="${module//\//-}"
  if [[ "$slug" == "." ]]; then
    slug="root"
  fi
  local module_log="$LOG_DIR/module-$slug-$STAMP.log"

  echo ""
  echo "========== go test: $module =========="
  echo "========== go test: $module ==========" >>"$module_log"

  if (
    cd "$ROOT/$module"
    if [[ "$module" == "web-server" ]]; then
      mkdir -p web/dist
      touch web/dist/index.html
    fi
    go test -count=1 ./... 2>&1 | tee -a "$module_log"
  ); then
    echo "- [pass] \`$module\`" >>"$SUMMARY_FILE"
    return 0
  fi

  failed=1
  failed_modules+=("$module")
  echo "- [fail] \`$module\` (see module-$slug-$STAMP.log)" >>"$SUMMARY_FILE"
  {
    echo ""
    echo "===== failures in $module ====="
    rg -n '^(--- FAIL|FAIL\t|panic:|    chat_test\.go|    .*_test\.go)' "$module_log" 2>/dev/null || \
      grep -En '^(--- FAIL|FAIL	|panic:)' "$module_log" || true
  } >>"$FAILURES_FILE"
  return 1
}

{
  echo "Writing logs to $LOG_DIR"
  echo "Main log: $LOG_FILE"
  echo "Failures: $FAILURES_FILE"
  echo "Summary: $SUMMARY_FILE"
} | tee "$LOG_FILE"

for module in "${MODULES[@]}"; do
  if ! run_module_tests "$module"; then
    echo "module failed: $module" | tee -a "$LOG_FILE"
  fi
done

{
  echo ""
  echo "## Result"
  if [[ "$failed" -eq 0 ]]; then
    echo "All modules passed."
  else
    echo "Failed modules:"
    for module in "${failed_modules[@]}"; do
      echo "- $module"
    done
    echo ""
    echo "## Failure excerpt"
    echo '```'
    cat "$FAILURES_FILE"
    echo '```'
  fi
} | tee -a "$SUMMARY_FILE" >>"$LOG_FILE"

ln -sf "$(basename "$LOG_FILE")" "$LOG_DIR/test-latest.log"
ln -sf "$(basename "$FAILURES_FILE")" "$LOG_DIR/failures-latest.txt"
ln -sf "$(basename "$SUMMARY_FILE")" "$LOG_DIR/summary-latest.md"

if [[ "$failed" -ne 0 ]]; then
  echo "CI tests failed. See $SUMMARY_FILE and $FAILURES_FILE" | tee -a "$LOG_FILE"
  exit 1
fi

echo "All CI tests passed." | tee -a "$LOG_FILE"
