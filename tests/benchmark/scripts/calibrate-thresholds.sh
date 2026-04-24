#!/bin/bash
#
# Calibrate threshold recommendations for find and recovery.
#
# Usage:
#   ./calibrate-thresholds.sh [--corpus <dir>]
#
# Reports recall/precision/false-positive-rate by threshold.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CORPUS_DIR="${BENCHMARK_DIR}/corpus"
RESULTS_DIR="${BENCHMARK_DIR}/results"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"

# Read config
if [[ -f "$CONFIG_FILE" ]]; then
    STRATEGY=$(jq -r '.defaults.strategy // "combined"' "$CONFIG_FILE")
    LEXICAL_WEIGHT=$(jq -r '.defaults.weights.lexical // 0.6' "$CONFIG_FILE")
    EMBEDDING_WEIGHT=$(jq -r '.defaults.weights.embedding // 0.4' "$CONFIG_FILE")
else
    STRATEGY="combined"
    LEXICAL_WEIGHT=0.6
    EMBEDDING_WEIGHT=0.4
fi

SPECIFIC_CORPUS=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --corpus) SPECIFIC_CORPUS="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

mkdir -p "${RESULTS_DIR}"

# Build semantic binary
echo "Building semantic..."
(cd "${BENCHMARK_DIR}/../.." && go build -o "${BENCHMARK_DIR}/semantic" ./cmd/semantic)

SEMANTIC="${BENCHMARK_DIR}/semantic"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/threshold_calibration_${TIMESTAMP}.json"

# Thresholds to test
THRESHOLDS=(0.01 0.05 0.10 0.15 0.20 0.25 0.30 0.35 0.40 0.45 0.50 0.60 0.70 0.80 0.90)

echo "Testing ${#THRESHOLDS[@]} thresholds: ${THRESHOLDS[*]}"
echo ""

# Initialize report
jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg strategy "${STRATEGY}" \
    '{
        timestamp: $ts,
        strategy: $strategy,
        thresholds: [],
        recommendations: {}
    }' > "${REPORT_FILE}"

# Collect results for each threshold
for thresh in "${THRESHOLDS[@]}"; do
    echo "Testing threshold: ${thresh}"

    total=0
    true_positives=0
    false_positives=0
    false_negatives=0

    for corpus in "${CORPUS_DIR}"/*/; do
        [[ -d "$corpus" ]] || continue

        if [[ -n "$SPECIFIC_CORPUS" ]] && [[ "$(basename "$corpus")" != "$SPECIFIC_CORPUS" ]]; then
            continue
        fi

        snapshot="${corpus}/snapshot.json"
        queries="${corpus}/queries.json"

        [[ -f "$snapshot" ]] && [[ -f "$queries" ]] || continue

        count=$(jq length "$queries")

        for i in $(seq 0 $((count - 1))); do
            query=$(jq -r ".[$i].query" "$queries")
            relevant_refs=$(jq -c ".[$i].relevant_refs" "$queries")

            result=$("${SEMANTIC}" find "${query}" \
                --snapshot "${snapshot}" \
                --strategy "${STRATEGY}" \
                --threshold "${thresh}" \
                --top-k 5 \
                --lexical-weight "${LEXICAL_WEIGHT}" \
                --embedding-weight "${EMBEDDING_WEIGHT}" \
                --format json 2>/dev/null) || continue

            best_ref=$(echo "$result" | jq -r '.best_ref // ""')
            num_matches=$(echo "$result" | jq '.matches | length')

            total=$((total + 1))

            # Check if best match is relevant
            if [[ -n "$best_ref" ]] && echo "$relevant_refs" | jq -e "index(\"${best_ref}\")" > /dev/null 2>&1; then
                true_positives=$((true_positives + 1))
            elif [[ -n "$best_ref" ]] && [[ "$num_matches" -gt 0 ]]; then
                false_positives=$((false_positives + 1))
            fi

            # If no match but there should be one
            if [[ -z "$best_ref" ]] || [[ "$num_matches" -eq 0 ]]; then
                rel_count=$(echo "$relevant_refs" | jq 'length')
                if [[ "$rel_count" -gt 0 ]]; then
                    false_negatives=$((false_negatives + 1))
                fi
            fi
        done
    done

    # Calculate metrics
    if [[ $total -eq 0 ]]; then
        echo "  No queries processed"
        continue
    fi

    precision=0
    recall=0
    fpr=0

    if [[ $((true_positives + false_positives)) -gt 0 ]]; then
        precision=$(echo "scale=4; $true_positives / ($true_positives + $false_positives)" | bc)
    fi

    if [[ $((true_positives + false_negatives)) -gt 0 ]]; then
        recall=$(echo "scale=4; $true_positives / ($true_positives + $false_negatives)" | bc)
    fi

    if [[ $((false_positives + true_positives)) -gt 0 ]]; then
        fpr=$(echo "scale=4; $false_positives / $total" | bc)
    fi

    f1=0
    if (( $(echo "$precision + $recall > 0" | bc -l) )); then
        f1=$(echo "scale=4; 2 * $precision * $recall / ($precision + $recall)" | bc)
    fi

    printf "  Precision: %.3f | Recall: %.3f | FPR: %.3f | F1: %.3f\n" "$precision" "$recall" "$fpr" "$f1"

    # Append to report
    tmp=$(mktemp)
    jq --argjson thresh "$thresh" \
       --argjson total "$total" \
       --argjson tp "$true_positives" \
       --argjson fp "$false_positives" \
       --argjson fn "$false_negatives" \
       --argjson precision "$precision" \
       --argjson recall "$recall" \
       --argjson fpr "$fpr" \
       --argjson f1 "$f1" \
       '.thresholds += [{
           threshold: $thresh,
           total: $total,
           true_positives: $tp,
           false_positives: $fp,
           false_negatives: $fn,
           precision: $precision,
           recall: $recall,
           false_positive_rate: $fpr,
           f1: $f1
       }]' "$REPORT_FILE" > "$tmp"
    mv "$tmp" "$REPORT_FILE"
done

# Calculate recommendations
echo ""
echo "Calculating recommendations..."

# Best F1 for general find
BEST_FIND=$(jq -r '[.thresholds[] | select(.f1 > 0)] | max_by(.f1) | .threshold // 0.3' "$REPORT_FILE")

# Best recall with precision > 0.8 for recovery (prioritize not missing)
BEST_RECOVERY=$(jq -r '[.thresholds[] | select(.precision >= 0.7)] | max_by(.recall) | .threshold // 0.2' "$REPORT_FILE")

# Update recommendations
tmp=$(mktemp)
jq --argjson find "$BEST_FIND" \
   --argjson recovery "$BEST_RECOVERY" \
   '.recommendations = {
       find: $find,
       recovery: $recovery,
       note: "find optimizes F1; recovery optimizes recall with precision >= 0.7"
   }' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

# Cleanup
rm -f "${BENCHMARK_DIR}/semantic"

echo ""
echo "================================================"
echo "  THRESHOLD CALIBRATION RESULTS"
echo "================================================"
echo "  Recommended for Find:     ${BEST_FIND}"
echo "  Recommended for Recovery: ${BEST_RECOVERY}"
echo "================================================"
echo ""
echo "Report: ${REPORT_FILE}"
