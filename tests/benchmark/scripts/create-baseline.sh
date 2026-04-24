#!/bin/bash
#
# Create a quality baseline from current corpus benchmark results.
#
# Usage:
#   ./create-baseline.sh [--name <name>]
#
# This runs run-corpus-benchmark.sh and saves the results as a baseline.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
BASELINES_DIR="${BENCHMARK_DIR}/baselines"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"

# Read defaults from config
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

STRATEGY=$(jq -r '.defaults.strategy // "combined"' "$CONFIG_FILE")

# Parse args
BASELINE_NAME="${STRATEGY}"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --name) BASELINE_NAME="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

mkdir -p "${BASELINES_DIR}"

BASELINE_FILE="${BASELINES_DIR}/${BASELINE_NAME}.json"

echo "Creating baseline: ${BASELINE_NAME}"
echo "Strategy: ${STRATEGY}"
echo ""

# Run corpus benchmark
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

"${SCRIPT_DIR}/run-corpus-benchmark.sh" --strategy "${STRATEGY}" 2>&1 | tee "${TEMP_DIR}/output.log"

# Find the latest report
LATEST_REPORT=$(ls -t "${BENCHMARK_DIR}/results"/corpus_${STRATEGY}_*.json 2>/dev/null | head -1)

if [[ -z "$LATEST_REPORT" ]] || [[ ! -f "$LATEST_REPORT" ]]; then
    echo "ERROR: Could not find benchmark report" >&2
    exit 1
fi

# Extract baseline data
jq '{
    created_at: .benchmark.timestamp,
    strategy: .benchmark.strategy,
    threshold: .benchmark.threshold,
    top_k: .benchmark.top_k,
    weights: .benchmark.weights,
    metrics: {
        total: .metrics.total,
        mrr: .metrics.mrr,
        p_at_1: .metrics.p_at_1,
        p_at_3: .metrics.p_at_3,
        hit_at_3: .metrics.hit_at_3,
        hit_at_5: .metrics.hit_at_5,
        avg_margin: .metrics.avg_margin,
        latency_p50_ms: .metrics.latency_p50_ms,
        latency_p95_ms: .metrics.latency_p95_ms
    },
    by_difficulty: .metrics.by_difficulty,
    by_corpus: .metrics.by_corpus,
    per_query: [.results[] | {id, corpus, difficulty, p_at_1, rr, margin}]
}' "$LATEST_REPORT" > "$BASELINE_FILE"

echo ""
echo "================================================"
echo "  BASELINE CREATED"
echo "================================================"
echo "  File: ${BASELINE_FILE}"
echo ""
jq -r '"  MRR:     \(.metrics.mrr)\n  P@1:     \(.metrics.p_at_1)\n  Hit@3:   \(.metrics.hit_at_3)\n  Margin:  \(.metrics.avg_margin)"' "$BASELINE_FILE"
echo "================================================"
