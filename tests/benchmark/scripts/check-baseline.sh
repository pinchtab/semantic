#!/bin/bash
#
# Check current benchmark results against a baseline.
#
# Usage:
#   ./check-baseline.sh [--baseline <file>] [--fail-on-regression]
#
# Exit codes:
#   0 - No regressions detected
#   1 - Regressions detected (if --fail-on-regression)
#   2 - Error (missing files, invalid config)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
BASELINES_DIR="${BENCHMARK_DIR}/baselines"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# Read config
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE" >&2
    exit 2
fi

STRATEGY=$(jq -r '.defaults.strategy // "combined"' "$CONFIG_FILE")
MAX_P1_DROP=$(jq -r '.baseline.quality.max_overall_p_at_1_drop // 0.02' "$CONFIG_FILE")
MAX_MRR_DROP=$(jq -r '.baseline.quality.max_overall_mrr_drop // 0.02' "$CONFIG_FILE")
MAX_HIT3_DROP=$(jq -r '.baseline.quality.max_overall_hit_at_3_drop // 0.02' "$CONFIG_FILE")
MAX_CORPUS_P1_DROP=$(jq -r '.baseline.quality.max_corpus_p_at_1_drop // 0.08' "$CONFIG_FILE")
MAX_MARGIN_DROP=$(jq -r '.baseline.quality.max_margin_drop_report // 0.15' "$CONFIG_FILE")

# Parse args
BASELINE_FILE="${BASELINES_DIR}/${STRATEGY}.json"
FAIL_ON_REGRESSION=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --baseline) BASELINE_FILE="$2"; shift 2 ;;
        --fail-on-regression) FAIL_ON_REGRESSION=true; shift ;;
        *) echo "Unknown option: $1"; exit 2 ;;
    esac
done

if [[ ! -f "$BASELINE_FILE" ]]; then
    echo "ERROR: Baseline not found: $BASELINE_FILE" >&2
    echo "Run ./create-baseline.sh first" >&2
    exit 2
fi

echo "Checking against baseline: ${BASELINE_FILE}"
echo "Tolerances: P@1=${MAX_P1_DROP}, MRR=${MAX_MRR_DROP}, Hit@3=${MAX_HIT3_DROP}"
echo ""

# Run current benchmark
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

"${SCRIPT_DIR}/run-corpus-benchmark.sh" --strategy "${STRATEGY}" > "${TEMP_DIR}/output.log" 2>&1

# Find the latest report
LATEST_REPORT=$(ls -t "${BENCHMARK_DIR}/results"/corpus_${STRATEGY}_*.json 2>/dev/null | head -1)

if [[ -z "$LATEST_REPORT" ]] || [[ ! -f "$LATEST_REPORT" ]]; then
    echo "ERROR: Could not find benchmark report" >&2
    exit 2
fi

# Compare metrics
REGRESSIONS=0
WARNINGS=0

compare_metric() {
    local name="$1"
    local baseline_val="$2"
    local current_val="$3"
    local max_drop="$4"

    local diff
    diff=$(echo "scale=4; $current_val - $baseline_val" | bc)
    local drop
    drop=$(echo "scale=4; $baseline_val - $current_val" | bc)

    if (( $(echo "$drop > $max_drop" | bc -l) )); then
        echo -e "${RED}REGRESSION${NC} $name: $baseline_val -> $current_val (drop: $drop, max: $max_drop)"
        REGRESSIONS=$((REGRESSIONS + 1))
    elif (( $(echo "$drop > 0" | bc -l) )); then
        echo -e "${YELLOW}WARNING${NC} $name: $baseline_val -> $current_val (drop: $drop)"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}OK${NC} $name: $baseline_val -> $current_val (${diff:0:6})"
    fi
}

echo "=== Overall Metrics ==="
echo ""

BASELINE_MRR=$(jq -r '.metrics.mrr' "$BASELINE_FILE")
CURRENT_MRR=$(jq -r '.metrics.mrr' "$LATEST_REPORT")
compare_metric "MRR" "$BASELINE_MRR" "$CURRENT_MRR" "$MAX_MRR_DROP"

BASELINE_P1=$(jq -r '.metrics.p_at_1' "$BASELINE_FILE")
CURRENT_P1=$(jq -r '.metrics.p_at_1' "$LATEST_REPORT")
compare_metric "P@1" "$BASELINE_P1" "$CURRENT_P1" "$MAX_P1_DROP"

BASELINE_HIT3=$(jq -r '.metrics.hit_at_3' "$BASELINE_FILE")
CURRENT_HIT3=$(jq -r '.metrics.hit_at_3' "$LATEST_REPORT")
compare_metric "Hit@3" "$BASELINE_HIT3" "$CURRENT_HIT3" "$MAX_HIT3_DROP"

BASELINE_MARGIN=$(jq -r '.metrics.avg_margin' "$BASELINE_FILE")
CURRENT_MARGIN=$(jq -r '.metrics.avg_margin' "$LATEST_REPORT")
compare_metric "Margin" "$BASELINE_MARGIN" "$CURRENT_MARGIN" "$MAX_MARGIN_DROP"

echo ""
echo "=== Per-Corpus ==="
echo ""

for corpus in $(jq -r '.by_corpus | keys[]' "$BASELINE_FILE"); do
    BASELINE_CORPUS_P1=$(jq -r ".by_corpus[\"$corpus\"].p_at_1 // 0" "$BASELINE_FILE")
    CURRENT_CORPUS_P1=$(jq -r ".metrics.by_corpus[\"$corpus\"].p_at_1 // 0" "$LATEST_REPORT")
    compare_metric "$corpus P@1" "$BASELINE_CORPUS_P1" "$CURRENT_CORPUS_P1" "$MAX_CORPUS_P1_DROP"
done

echo ""
echo "================================================"
if [[ $REGRESSIONS -gt 0 ]]; then
    echo -e "${RED}REGRESSIONS: $REGRESSIONS${NC}"
    if [[ "$FAIL_ON_REGRESSION" == "true" ]]; then
        exit 1
    fi
elif [[ $WARNINGS -gt 0 ]]; then
    echo -e "${YELLOW}WARNINGS: $WARNINGS (no regressions)${NC}"
else
    echo -e "${GREEN}ALL CHECKS PASSED${NC}"
fi
echo "================================================"
