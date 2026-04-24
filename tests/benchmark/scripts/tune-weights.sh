#!/bin/bash
#
# Grid-search combined matcher lexical/embedding weights against the corpus.
#
# Usage:
#   ./tune-weights.sh [--corpus <dir>] [--step <n>] [--output <dir>]
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
RESULTS_DIR="${BENCHMARK_DIR}/results"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"

# Read defaults from config (used for threshold/top_k in grid runs)
if [[ -f "$CONFIG_FILE" ]]; then
    THRESHOLD=$(jq -r '.defaults.threshold // 0.01' "$CONFIG_FILE")
    TOP_K=$(jq -r '.defaults.top_k // 5' "$CONFIG_FILE")
else
    THRESHOLD=0.01
    TOP_K=5
fi

SPECIFIC_CORPUS=""
STEP="0.1"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --corpus) SPECIFIC_CORPUS="$2"; shift 2 ;;
        --step) STEP="$2"; shift 2 ;;
        --output) RESULTS_DIR="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

mkdir -p "${RESULTS_DIR}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/tuning_weights_${TIMESTAMP}.json"
SUMMARY_FILE="${REPORT_FILE%.json}_summary.md"

jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg step "${STEP}" \
    '{
        benchmark: {
            timestamp: $ts,
            type: "weight-tuning",
            strategy: "combined",
            step: ($step | tonumber)
        },
        results: [],
        best: null
    }' > "${REPORT_FILE}"

weights=$(awk -v step="${STEP}" 'BEGIN {
    if (step <= 0 || step > 1) {
        exit 1
    }
    for (w = 0; w <= 1.000001; w += step) {
        printf "%.4f\n", w
    }
}')

if [[ -z "${weights}" ]]; then
    echo "Invalid step: ${STEP}" >&2
    exit 1
fi

echo "Weight tuning: step=${STEP}"
echo ""
printf "%-10s %-10s %-8s %-8s %-8s %-8s %-8s\n" "lexical" "embedding" "MRR" "P@1" "P@3" "P50" "report"

while IFS= read -r lexical_weight; do
    embedding_weight=$(awk -v w="${lexical_weight}" 'BEGIN { printf "%.4f", 1 - w }')

    args=(
        --strategy combined
        --lexical-weight "${lexical_weight}"
        --embedding-weight "${embedding_weight}"
    )
    if [[ -n "${SPECIFIC_CORPUS}" ]]; then
        args+=(--corpus "${SPECIFIC_CORPUS}")
    fi

    if ! output=$("${SCRIPT_DIR}/run-corpus-benchmark.sh" "${args[@]}" 2>&1); then
        echo "$output" >&2
        exit 1
    fi

    corpus_report=$(echo "$output" | awk '/^Report:/ {print $2}' | tail -1)
    if [[ -z "${corpus_report}" || ! -f "${corpus_report}" ]]; then
        echo "Could not find corpus report for lexical=${lexical_weight}" >&2
        echo "$output" >&2
        exit 1
    fi

    mrr=$(jq -r '.metrics.mrr' "$corpus_report")
    p1=$(jq -r '.metrics.p_at_1' "$corpus_report")
    p3=$(jq -r '.metrics.p_at_3' "$corpus_report")
    p50=$(jq -r '.metrics.latency_p50_ms' "$corpus_report")
    total=$(jq -r '.metrics.total' "$corpus_report")

    printf "%-10s %-10s %-8s %-8s %-8s %-8s %s\n" \
        "${lexical_weight}" "${embedding_weight}" "${mrr}" "${p1}" "${p3}" "${p50}" "$(basename "$corpus_report")"

    result_json=$(jq -n \
        --argjson lexical_weight "${lexical_weight}" \
        --argjson embedding_weight "${embedding_weight}" \
        --argjson total "${total}" \
        --argjson mrr "${mrr}" \
        --argjson p1 "${p1}" \
        --argjson p3 "${p3}" \
        --argjson p50 "${p50}" \
        --arg report "${corpus_report}" \
        '{
            lexical_weight: $lexical_weight,
            embedding_weight: $embedding_weight,
            total: $total,
            mrr: $mrr,
            p_at_1: $p1,
            p_at_3: $p3,
            latency_p50_ms: $p50,
            report: $report
        }')

    tmp=$(mktemp)
    jq --argjson result "${result_json}" '.results += [$result]' "${REPORT_FILE}" > "$tmp"
    mv "$tmp" "${REPORT_FILE}"
done <<< "${weights}"

tmp=$(mktemp)
jq '
    .best = (
        .results
        | sort_by(.p_at_1, .mrr, .p_at_3, -(.latency_p50_ms))
        | last
    )
' "${REPORT_FILE}" > "$tmp"
mv "$tmp" "${REPORT_FILE}"

cat > "${SUMMARY_FILE}" << EOF
# Combined Weight Tuning

## Best

| Field | Value |
|-------|-------|
| Lexical Weight | $(jq -r '.best.lexical_weight' "$REPORT_FILE") |
| Embedding Weight | $(jq -r '.best.embedding_weight' "$REPORT_FILE") |
| MRR | $(jq -r '.best.mrr' "$REPORT_FILE") |
| P@1 | $(jq -r '.best.p_at_1' "$REPORT_FILE") |
| P@3 | $(jq -r '.best.p_at_3' "$REPORT_FILE") |
| Latency P50 | $(jq -r '.best.latency_p50_ms' "$REPORT_FILE") ms |

## All Runs

| Lexical | Embedding | MRR | P@1 | P@3 | P50 |
|---------|-----------|-----|-----|-----|-----|
$(jq -r '.results | sort_by(-.p_at_1, -.mrr, -.p_at_3, .latency_p50_ms)[] | "| \(.lexical_weight) | \(.embedding_weight) | \(.mrr) | \(.p_at_1) | \(.p_at_3) | \(.latency_p50_ms) ms |"' "$REPORT_FILE")
EOF

echo ""
echo "Best weights:"
jq '.best' "${REPORT_FILE}"
echo ""
echo "Report:  ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
