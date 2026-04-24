#!/bin/bash
#
# Check Go benchmark results against runtime baseline.
#
# Usage:
#   ./check-runtime-baseline.sh [--fail-on-regression]
#
# Runs Go benchmarks and compares against saved baseline.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
BASELINES_DIR="${BENCHMARK_DIR}/baselines"
RESULTS_DIR="${BENCHMARK_DIR}/results"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"
PROJECT_ROOT="${BENCHMARK_DIR}/../.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

# Read tolerances from config
if [[ -f "$CONFIG_FILE" ]]; then
    MAX_NS_RATIO=$(jq -r '.baseline.runtime.max_ns_op_regression_ratio // 1.25' "$CONFIG_FILE")
    MAX_ALLOC_RATIO=$(jq -r '.baseline.runtime.max_alloc_regression_ratio // 1.25' "$CONFIG_FILE")
else
    MAX_NS_RATIO=1.25
    MAX_ALLOC_RATIO=1.25
fi

# Parse args
FAIL_ON_REGRESSION=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --fail-on-regression) FAIL_ON_REGRESSION=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

mkdir -p "${RESULTS_DIR}"
mkdir -p "${BASELINES_DIR}"

BASELINE_FILE="${BASELINES_DIR}/runtime.json"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/runtime_${TIMESTAMP}.json"

echo "Running Go benchmarks..."
echo ""

# Run benchmarks
BENCH_OUTPUT=$(mktemp)
(cd "$PROJECT_ROOT" && go test -bench=. -benchmem ./internal/engine/... 2>&1) | tee "$BENCH_OUTPUT"

# Parse benchmark output into JSON
echo ""
echo "Parsing results..."

jq -n --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{timestamp: $ts, benchmarks: []}' > "$REPORT_FILE"

while IFS= read -r line; do
    if [[ "$line" =~ ^Benchmark ]]; then
        # Parse: BenchmarkName-N  iterations  ns/op  bytes/op  allocs/op
        name=$(echo "$line" | awk '{print $1}' | sed 's/-[0-9]*$//')
        ns_op=$(echo "$line" | grep -oE '[0-9.]+ ns/op' | awk '{print $1}' || echo "0")
        bytes_op=$(echo "$line" | grep -oE '[0-9]+ B/op' | awk '{print $1}' || echo "0")
        allocs_op=$(echo "$line" | grep -oE '[0-9]+ allocs/op' | awk '{print $1}' || echo "0")

        if [[ -n "$ns_op" ]] && [[ "$ns_op" != "0" ]]; then
            tmp=$(mktemp)
            jq --arg name "$name" \
               --argjson ns "$ns_op" \
               --argjson bytes "${bytes_op:-0}" \
               --argjson allocs "${allocs_op:-0}" \
               '.benchmarks += [{name: $name, ns_op: $ns, bytes_op: $bytes, allocs_op: $allocs}]' \
               "$REPORT_FILE" > "$tmp"
            mv "$tmp" "$REPORT_FILE"
        fi
    fi
done < "$BENCH_OUTPUT"

rm -f "$BENCH_OUTPUT"

# If no baseline exists, create one
if [[ ! -f "$BASELINE_FILE" ]]; then
    echo ""
    echo "No runtime baseline found. Creating initial baseline..."
    cp "$REPORT_FILE" "$BASELINE_FILE"
    echo "Baseline saved to: $BASELINE_FILE"
    exit 0
fi

# Compare against baseline
echo ""
echo "=== Comparing against baseline ==="
echo ""

REGRESSIONS=0

for name in $(jq -r '.benchmarks[].name' "$REPORT_FILE"); do
    baseline_ns=$(jq -r ".benchmarks[] | select(.name == \"$name\") | .ns_op // 0" "$BASELINE_FILE")
    current_ns=$(jq -r ".benchmarks[] | select(.name == \"$name\") | .ns_op // 0" "$REPORT_FILE")

    baseline_allocs=$(jq -r ".benchmarks[] | select(.name == \"$name\") | .allocs_op // 0" "$BASELINE_FILE")
    current_allocs=$(jq -r ".benchmarks[] | select(.name == \"$name\") | .allocs_op // 0" "$REPORT_FILE")

    if [[ "$baseline_ns" == "0" ]] || [[ "$baseline_ns" == "null" ]]; then
        echo -e "${YELLOW}NEW${NC} $name: ${current_ns} ns/op"
        continue
    fi

    ratio=$(echo "scale=4; $current_ns / $baseline_ns" | bc)

    if (( $(echo "$ratio > $MAX_NS_RATIO" | bc -l) )); then
        echo -e "${RED}REGRESSION${NC} $name: ${baseline_ns} -> ${current_ns} ns/op (${ratio}x, max: ${MAX_NS_RATIO}x)"
        REGRESSIONS=$((REGRESSIONS + 1))
    elif (( $(echo "$ratio > 1.1" | bc -l) )); then
        echo -e "${YELLOW}WARNING${NC} $name: ${baseline_ns} -> ${current_ns} ns/op (${ratio}x)"
    else
        echo -e "${GREEN}OK${NC} $name: ${baseline_ns} -> ${current_ns} ns/op (${ratio}x)"
    fi
done

echo ""
echo "================================================"
if [[ $REGRESSIONS -gt 0 ]]; then
    echo -e "${RED}RUNTIME REGRESSIONS: $REGRESSIONS${NC}"
    if [[ "$FAIL_ON_REGRESSION" == "true" ]]; then
        exit 1
    fi
else
    echo -e "${GREEN}NO RUNTIME REGRESSIONS${NC}"
fi
echo "================================================"
echo ""
echo "Report: ${REPORT_FILE}"
