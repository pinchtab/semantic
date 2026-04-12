#!/bin/bash
#
# Record a benchmark result
#
# Usage:
#   ./record-result.sh <report_file> <id> <pass|fail|skip> <score> <latency_ms> "notes"
#
set -euo pipefail

if [[ $# -lt 5 ]]; then
    echo "Usage: $0 <report_file> <id> <pass|fail|skip> <score> <latency_ms> [notes]"
    exit 1
fi

REPORT_FILE="$1"
ID="$2"
STATUS="$3"
SCORE="$4"
LATENCY_MS="$5"
NOTES="${6:-}"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Create result entry
RESULT_JSON=$(jq -n \
    --arg id "${ID}" \
    --arg status "${STATUS}" \
    --argjson score "${SCORE}" \
    --argjson latency "${LATENCY_MS}" \
    --arg notes "${NOTES}" \
    --arg ts "${TIMESTAMP}" \
    '{id: $id, status: $status, score: $score, latency_ms: $latency, notes: $notes, timestamp: $ts}')

# Append to report
TMP_FILE=$(mktemp)
jq --argjson result "${RESULT_JSON}" \
   --arg status "${STATUS}" \
   '.results += [$result] |
    .summary.total += 1 |
    if $status == "pass" then .summary.passed += 1
    elif $status == "fail" then .summary.failed += 1
    else .summary.skipped += 1 end' \
   "${REPORT_FILE}" > "${TMP_FILE}"

mv "${TMP_FILE}" "${REPORT_FILE}"
