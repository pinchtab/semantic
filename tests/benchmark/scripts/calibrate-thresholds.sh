#!/bin/bash
#
# Threshold Calibration Benchmark
#
# Calculates optimal thresholds for semantic matching by evaluating
# recall, precision, and false-positive rates across threshold levels.
#
# Usage:
#   ./calibrate-thresholds.sh [--corpus <dir>]
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CORPUS_DIR="${BENCHMARK_DIR}/corpus"
CASES_DIR="${BENCHMARK_DIR}/cases"
RESULTS_DIR="${BENCHMARK_DIR}/results"

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
THRESHOLDS=(0.05 0.10 0.15 0.20 0.25 0.30 0.35 0.40 0.45 0.50 0.55 0.60)

# Initialize report
jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson thresholds "$(printf '%s\n' "${THRESHOLDS[@]}" | jq -s '.')" \
    '{
        calibration: {
            timestamp: $ts,
            thresholds_tested: $thresholds
        },
        by_threshold: {},
        by_tag: {},
        recommendations: {}
    }' > "${REPORT_FILE}"

echo ""
echo "=== Threshold Calibration ==="
echo "Testing thresholds: ${THRESHOLDS[*]}"
echo ""

# Collect all test cases
declare -a ALL_QUERIES=()
declare -a ALL_SNAPSHOTS=()
declare -a ALL_RELEVANT=()
declare -a ALL_EXPECT_NO_MATCH=()
declare -a ALL_IDS=()

load_corpus() {
    local corpus_path="$1"
    local snapshot="${corpus_path}/snapshot.json"
    local queries="${corpus_path}/queries.json"

    if [[ ! -f "$snapshot" ]] || [[ ! -f "$queries" ]]; then
        return
    fi

    local count
    count=$(jq length "$queries")

    for i in $(seq 0 $((count - 1))); do
        local query relevant id expect_no_match
        id=$(jq -r ".[$i].id" "$queries")
        query=$(jq -r ".[$i].query" "$queries")
        relevant=$(jq -c ".[$i].relevant_refs // []" "$queries")
        expect_no_match=$(jq -r ".[$i].expect_no_match // false" "$queries")

        ALL_IDS+=("$id")
        ALL_QUERIES+=("$query")
        ALL_SNAPSHOTS+=("$snapshot")
        ALL_RELEVANT+=("$relevant")
        ALL_EXPECT_NO_MATCH+=("$expect_no_match")
    done
}

load_cases() {
    local cases_file="$1"
    local snapshots_dir="${BENCHMARK_DIR}/../e2e/assets/snapshots"

    if [[ ! -f "$cases_file" ]]; then
        return
    fi

    local count
    count=$(jq length "$cases_file")

    for i in $(seq 0 $((count - 1))); do
        local id query snapshot_name expect_no_match expect_ref expect_ref_alt relevant
        id=$(jq -r ".[$i].id" "$cases_file")
        query=$(jq -r ".[$i].query" "$cases_file")
        snapshot_name=$(jq -r ".[$i].snapshot" "$cases_file")
        expect_no_match=$(jq -r ".[$i].expect_no_match // false" "$cases_file")
        expect_ref=$(jq -r ".[$i].expect_ref // \"\"" "$cases_file")
        expect_ref_alt=$(jq -c ".[$i].expect_ref_alt // []" "$cases_file")

        if [[ -n "$expect_ref" && "$expect_ref" != "null" ]]; then
            relevant=$(echo "$expect_ref_alt" | jq --arg r "$expect_ref" '. + [$r]')
        else
            relevant="[]"
        fi

        local snapshot="${snapshots_dir}/${snapshot_name}"
        if [[ ! -f "$snapshot" ]]; then
            continue
        fi

        ALL_IDS+=("$id")
        ALL_QUERIES+=("$query")
        ALL_SNAPSHOTS+=("$snapshot")
        ALL_RELEVANT+=("$relevant")
        ALL_EXPECT_NO_MATCH+=("$expect_no_match")
    done
}

echo "Loading test cases..."
if [[ -n "${SPECIFIC_CORPUS}" ]]; then
    load_corpus "${CORPUS_DIR}/${SPECIFIC_CORPUS}"
else
    for corpus in "${CORPUS_DIR}"/*/; do
        [[ -d "$corpus" ]] || continue
        load_corpus "$corpus"
    done
fi

load_cases "${CASES_DIR}/negative-threshold.json"

TOTAL_CASES=${#ALL_QUERIES[@]}
echo "Loaded ${TOTAL_CASES} test cases"
echo ""

for threshold in "${THRESHOLDS[@]}"; do
    echo "Testing threshold ${threshold}..."

    tp=0 fp=0 fn=0 tn=0

    for i in $(seq 0 $((TOTAL_CASES - 1))); do
        query="${ALL_QUERIES[$i]}"
        snapshot="${ALL_SNAPSHOTS[$i]}"
        relevant="${ALL_RELEVANT[$i]}"
        expect_no_match="${ALL_EXPECT_NO_MATCH[$i]}"

        result=$("${SEMANTIC}" find "${query}" \
            --snapshot "${snapshot}" \
            --strategy combined \
            --threshold "${threshold}" \
            --top-k 5 \
            --format json 2>/dev/null) || result='{"matches":[]}'

        match_count=$(echo "$result" | jq '.matches | length')
        best_ref=$(echo "$result" | jq -r '.best_ref // ""')

        if [[ "$expect_no_match" == "true" ]]; then
            if [[ $match_count -eq 0 ]]; then
                tn=$((tn + 1))
            else
                fp=$((fp + 1))
            fi
        else
            relevant_count=$(echo "$relevant" | jq 'length')
            if [[ $relevant_count -eq 0 ]]; then
                continue
            fi

            if [[ $match_count -eq 0 ]]; then
                fn=$((fn + 1))
            elif echo "$relevant" | jq -e "index(\"${best_ref}\")" > /dev/null 2>&1; then
                tp=$((tp + 1))
            else
                fp=$((fp + 1))
            fi
        fi
    done

    total_positive=$((tp + fn))
    total_negative=$((tn + fp))

    if [[ $total_positive -gt 0 ]]; then
        recall=$(echo "scale=4; $tp / $total_positive" | bc)
    else
        recall="0"
    fi

    if [[ $((tp + fp)) -gt 0 ]]; then
        precision=$(echo "scale=4; $tp / ($tp + $fp)" | bc)
    else
        precision="1"
    fi

    if [[ $total_negative -gt 0 ]]; then
        fpr=$(echo "scale=4; $fp / $total_negative" | bc)
    else
        fpr="0"
    fi

    if [[ $(echo "$precision + $recall > 0" | bc) -eq 1 ]]; then
        f1=$(echo "scale=4; 2 * $precision * $recall / ($precision + $recall)" | bc)
    else
        f1="0"
    fi

    printf "  threshold=%.2f | TP=%3d FP=%3d FN=%3d TN=%3d | recall=%.3f precision=%.3f FPR=%.3f F1=%.3f\n" \
        "$threshold" "$tp" "$fp" "$fn" "$tn" "$recall" "$precision" "$fpr" "$f1"

    tmp=$(mktemp)
    jq --arg t "$threshold" \
       --argjson tp "$tp" --argjson fp "$fp" --argjson fn "$fn" --argjson tn "$tn" \
       --argjson recall "$recall" --argjson precision "$precision" \
       --argjson fpr "$fpr" --argjson f1 "$f1" \
       '.by_threshold[$t] = {
           tp: $tp, fp: $fp, fn: $fn, tn: $tn,
           recall: $recall, precision: $precision,
           false_positive_rate: $fpr, f1: $f1
       }' "$REPORT_FILE" > "$tmp"
    mv "$tmp" "$REPORT_FILE"
done

echo ""
echo "Calculating recommendations..."

best_f1_threshold="" best_f1=0
best_recall_threshold="" best_recall=0

for threshold in "${THRESHOLDS[@]}"; do
    metrics=$(jq -r ".by_threshold[\"$threshold\"]" "$REPORT_FILE")
    f1=$(echo "$metrics" | jq -r '.f1')
    recall=$(echo "$metrics" | jq -r '.recall')

    if (( $(echo "$f1 > $best_f1" | bc -l) )); then
        best_f1=$f1
        best_f1_threshold=$threshold
    fi
    if (( $(echo "$recall > $best_recall" | bc -l) )); then
        best_recall=$recall
        best_recall_threshold=$threshold
    fi
done

recovery_threshold=""
recovery_precision=0
for threshold in "${THRESHOLDS[@]}"; do
    metrics=$(jq -r ".by_threshold[\"$threshold\"]" "$REPORT_FILE")
    recall=$(echo "$metrics" | jq -r '.recall')
    precision=$(echo "$metrics" | jq -r '.precision')

    if (( $(echo "$recall >= 0.85" | bc -l) )); then
        if (( $(echo "$precision > $recovery_precision" | bc -l) )); then
            recovery_precision=$precision
            recovery_threshold=$threshold
        fi
    fi
done

if [[ -z "$recovery_threshold" ]]; then
    recovery_threshold="${THRESHOLDS[0]}"
fi

default_threshold="$best_f1_threshold"

tmp=$(mktemp)
jq --arg default "$default_threshold" \
   --arg recovery "$recovery_threshold" \
   --arg best_f1 "$best_f1_threshold" \
   --argjson best_f1_val "$best_f1" \
   '.recommendations = {
       default_threshold: $default,
       recovery_threshold: $recovery,
       best_f1: { threshold: $best_f1, value: $best_f1_val },
       notes: "default_threshold optimizes F1. recovery_threshold prioritizes recall (>=85%)."
   }' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

SUMMARY_FILE="${REPORT_FILE%.json}_summary.md"

cat > "${SUMMARY_FILE}" << EOF
# Threshold Calibration Report

Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)

## Recommendations

| Use Case | Threshold | Rationale |
|----------|-----------|-----------|
| **Default (find)** | **${default_threshold}** | Best F1 score (${best_f1}) |
| **Recovery** | **${recovery_threshold}** | High recall for element recovery |

## Metrics by Threshold

| Threshold | TP | FP | FN | TN | Recall | Precision | FPR | F1 |
|-----------|----|----|----|----|--------|-----------|-----|-----|
$(for t in "${THRESHOLDS[@]}"; do
    m=$(jq -r ".by_threshold[\"$t\"]" "$REPORT_FILE")
    printf "| %.2f | %d | %d | %d | %d | %.3f | %.3f | %.3f | %.3f |\n" \
        "$t" \
        "$(echo "$m" | jq -r '.tp')" \
        "$(echo "$m" | jq -r '.fp')" \
        "$(echo "$m" | jq -r '.fn')" \
        "$(echo "$m" | jq -r '.tn')" \
        "$(echo "$m" | jq -r '.recall')" \
        "$(echo "$m" | jq -r '.precision')" \
        "$(echo "$m" | jq -r '.false_positive_rate')" \
        "$(echo "$m" | jq -r '.f1')"
done)

## Trade-offs

- **Lower threshold** (0.10-0.20): High recall, more false positives. Good for recovery.
- **Medium threshold** (0.25-0.35): Balanced. Good default for find operations.
- **Higher threshold** (0.40+): High precision, misses weaker matches.
EOF

rm -f "${BENCHMARK_DIR}/semantic"

echo ""
echo "================================================"
echo "  THRESHOLD CALIBRATION COMPLETE"
echo "================================================"
echo "  Test cases:         ${TOTAL_CASES}"
echo "  Default threshold:  ${default_threshold} (F1=${best_f1})"
echo "  Recovery threshold: ${recovery_threshold}"
echo "================================================"
echo ""
echo "Report:  ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
