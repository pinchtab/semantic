#!/bin/bash
#
# Finalize benchmark report and generate summary
#
# Usage:
#   ./finalize-report.sh <report_file>
#
set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <report_file>"
    exit 1
fi

REPORT_FILE="$1"
SUMMARY_FILE="${REPORT_FILE%.json}_summary.md"

# Calculate final metrics
TMP_FILE=$(mktemp)
jq '
    .summary.accuracy = (if .summary.total > 0 then (.summary.passed / .summary.total * 100 | floor / 100) else 0 end) |
    .summary.avg_score = (if (.results | length) > 0 then ([.results[].score] | add / length | . * 1000 | floor / 1000) else 0 end) |
    .summary.avg_latency_ms = (if (.results | length) > 0 then ([.results[].latency_ms] | add / length | floor) else 0 end) |
    .summary.min_score = (if (.results | length) > 0 then ([.results[].score] | min) else 0 end) |
    .summary.max_score = (if (.results | length) > 0 then ([.results[].score] | max) else 0 end) |
    .summary.min_latency_ms = (if (.results | length) > 0 then ([.results[].latency_ms] | min) else 0 end) |
    .summary.max_latency_ms = (if (.results | length) > 0 then ([.results[].latency_ms] | max) else 0 end)
' "${REPORT_FILE}" > "${TMP_FILE}"
mv "${TMP_FILE}" "${REPORT_FILE}"

# Generate markdown summary
TIMESTAMP=$(jq -r '.benchmark.timestamp' "${REPORT_FILE}")
STRATEGY=$(jq -r '.benchmark.strategy' "${REPORT_FILE}")
VERSION=$(jq -r '.benchmark.version' "${REPORT_FILE}")
TOTAL=$(jq -r '.summary.total' "${REPORT_FILE}")
PASSED=$(jq -r '.summary.passed' "${REPORT_FILE}")
FAILED=$(jq -r '.summary.failed' "${REPORT_FILE}")
SKIPPED=$(jq -r '.summary.skipped' "${REPORT_FILE}")
ACCURACY=$(jq -r '.summary.accuracy' "${REPORT_FILE}")
AVG_SCORE=$(jq -r '.summary.avg_score' "${REPORT_FILE}")
AVG_LATENCY=$(jq -r '.summary.avg_latency_ms' "${REPORT_FILE}")
MIN_SCORE=$(jq -r '.summary.min_score' "${REPORT_FILE}")
MAX_SCORE=$(jq -r '.summary.max_score' "${REPORT_FILE}")
MIN_LATENCY=$(jq -r '.summary.min_latency_ms' "${REPORT_FILE}")
MAX_LATENCY=$(jq -r '.summary.max_latency_ms' "${REPORT_FILE}")

cat > "${SUMMARY_FILE}" << EOF
# Semantic Matching Benchmark Results

## Benchmark Info

| Field | Value |
|-------|-------|
| Timestamp | ${TIMESTAMP} |
| Strategy | ${STRATEGY} |
| Version | ${VERSION} |

## Results Summary

| Metric | Value |
|--------|-------|
| Total Cases | ${TOTAL} |
| Passed | ${PASSED} |
| Failed | ${FAILED} |
| Skipped | ${SKIPPED} |
| **Accuracy** | **${ACCURACY}%** |

## Score Distribution

| Metric | Value |
|--------|-------|
| Average Score | ${AVG_SCORE} |
| Min Score | ${MIN_SCORE} |
| Max Score | ${MAX_SCORE} |

## Latency

| Metric | Value |
|--------|-------|
| Average | ${AVG_LATENCY} ms |
| Min | ${MIN_LATENCY} ms |
| Max | ${MAX_LATENCY} ms |

## Failed Cases

EOF

# Add failed cases
jq -r '.results[] | select(.status == "fail") | "| \(.id) | \(.notes) |"' "${REPORT_FILE}" >> "${SUMMARY_FILE}"

if [[ $(jq '[.results[] | select(.status == "fail")] | length' "${REPORT_FILE}") -eq 0 ]]; then
    echo "_No failures_" >> "${SUMMARY_FILE}"
else
    # Add header
    sed -i.bak '/## Failed Cases/a\
| ID | Notes |\
|-----|-------|' "${SUMMARY_FILE}"
    rm -f "${SUMMARY_FILE}.bak"
fi

echo ""
echo "================================================"
echo "  BENCHMARK SUMMARY"
echo "================================================"
echo "  Strategy:  ${STRATEGY}"
echo "  Total:     ${TOTAL}"
echo "  Passed:    ${PASSED}"
echo "  Failed:    ${FAILED}"
echo "  Accuracy:  ${ACCURACY}%"
echo "  Avg Score: ${AVG_SCORE}"
echo "  Avg Latency: ${AVG_LATENCY} ms"
echo "================================================"
echo ""
echo "Report: ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
