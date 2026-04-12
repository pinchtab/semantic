#!/bin/bash
set -euo pipefail

echo ""
echo "═══════════════════════════════════════════════════"
echo "  semantic E2E tests"
echo "═══════════════════════════════════════════════════"
echo ""

# Verify binary is available
if ! command -v semantic &>/dev/null; then
  echo "ERROR: semantic binary not found"
  exit 1
fi

TOTAL_PASSED=0
TOTAL_FAILED=0

run_suite() {
  local suite_file="$1"
  source "$suite_file"
  TOTAL_PASSED=$((TOTAL_PASSED + PASSED))
  TOTAL_FAILED=$((TOTAL_FAILED + FAILED))
}

# Run all test suites
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for suite in "${SCRIPT_DIR}"/cases/*.sh; do
  [ -f "$suite" ] || continue
  echo ""
  run_suite "$suite"
done

# Final summary
echo ""
echo "═══════════════════════════════════════════════════"
echo "  TOTAL"
echo "  Passed: $TOTAL_PASSED"
echo "  Failed: $TOTAL_FAILED"
echo "═══════════════════════════════════════════════════"

# Write aggregate results
if [ -d /results ]; then
  echo "passed=$TOTAL_PASSED" > /results/summary.txt
  echo "failed=$TOTAL_FAILED" >> /results/summary.txt
fi

if [ "$TOTAL_FAILED" -gt 0 ]; then
  exit 1
fi
