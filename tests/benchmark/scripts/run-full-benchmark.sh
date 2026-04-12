#!/bin/bash
#
# Full semantic benchmark: Find + Recovery + Classification
#
# Produces a composite score for overall system health.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CORPUS_DIR="${BENCHMARK_DIR}/corpus"
RESULTS_DIR="${BENCHMARK_DIR}/results"

mkdir -p "${RESULTS_DIR}"

# Build semantic binary with recovery support
echo "Building semantic..."
(cd "${BENCHMARK_DIR}/../.." && go build -o "${BENCHMARK_DIR}/semantic" ./cmd/semantic)

SEMANTIC="${BENCHMARK_DIR}/semantic"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/full_benchmark_${TIMESTAMP}.json"

has_role_keyword() {
    local query="$1"
    echo "$query" | grep -Eiq '(^|[^[:alnum:]])(button|input|link|textbox|checkbox|radio|select|option|tab|menu|form|search)([^[:alnum:]]|$)'
}

enrich_recovery_query() {
    local query="$1"
    local role="$2"

    if [[ -z "$query" || -z "$role" ]]; then
        printf '%s' "$query"
        return
    fi
    if has_role_keyword "$query"; then
        printf '%s' "$query"
        return
    fi
    printf '%s %s' "$query" "$role"
}

# Initialize report
jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '{
        timestamp: $ts,
        find: { total: 0, mrr: 0, p_at_1: 0, latency_p50: 0 },
        recovery: { total: 0, recovered: 0, rate: 0 },
        classification: { total: 0, correct: 0, accuracy: 0 },
        composite: { score: 0, grade: "" }
    }' > "${REPORT_FILE}"

echo ""
echo "=============================================="
echo "  PHASE 1: FIND BENCHMARK"
echo "=============================================="

# Run corpus benchmark and capture metrics
FIND_OUTPUT=$("${SCRIPT_DIR}/run-corpus-benchmark.sh" 2>&1)
echo "$FIND_OUTPUT"

# Extract metrics from output
FIND_MRR=$(echo "$FIND_OUTPUT" | grep "MRR:" | tail -1 | awk '{print $2}')
FIND_P1=$(echo "$FIND_OUTPUT" | grep "P@1:" | tail -1 | awk '{print $2}')
FIND_TOTAL=$(echo "$FIND_OUTPUT" | grep "Queries:" | tail -1 | awk '{print $2}')
FIND_LAT=$(echo "$FIND_OUTPUT" | grep "Latency P50:" | tail -1 | awk '{print $3}')

# Rebuild semantic binary (corpus benchmark deletes it)
(cd "${BENCHMARK_DIR}/../.." && go build -o "${BENCHMARK_DIR}/semantic" ./cmd/semantic)

echo ""
echo "=============================================="
echo "  PHASE 2: RECOVERY BENCHMARK"
echo "=============================================="

SCENARIOS_FILE="${CORPUS_DIR}/recovery-scenarios/scenarios.json"
RECOVERY_TOTAL=0
RECOVERY_SUCCESS=0

if [[ -f "$SCENARIOS_FILE" ]]; then
    SCENARIO_COUNT=$(jq length "$SCENARIOS_FILE")

    for i in $(seq 0 $((SCENARIO_COUNT - 1))); do
        ID=$(jq -r ".[$i].id" "$SCENARIOS_FILE")
        NAME=$(jq -r ".[$i].name" "$SCENARIOS_FILE")
        RAW_QUERY=$(jq -r ".[$i].original_query" "$SCENARIOS_FILE")
        ORIGINAL_REF=$(jq -r ".[$i].original_ref // empty" "$SCENARIOS_FILE")
        ORIGINAL_ROLE=$(jq -r ".[$i].before[]? | select(.ref == \"$ORIGINAL_REF\") | .role // empty" "$SCENARIOS_FILE")
        QUERY=$(enrich_recovery_query "$RAW_QUERY" "$ORIGINAL_ROLE")
        EXPECTED=$(jq -r ".[$i].expected_ref // empty" "$SCENARIOS_FILE")
        EXPECTED_ALT=$(jq -r ".[$i].expected_alt // [] | join(\",\")" "$SCENARIOS_FILE")
        EXPECT_NO_MATCH=$(jq -r ".[$i].expect_no_match // false" "$SCENARIOS_FILE")

        # Write after snapshot to temp file
        AFTER_FILE=$(mktemp)
        jq ".[$i].after" "$SCENARIOS_FILE" > "$AFTER_FILE"

        # Run semantic find on after snapshot with the same minimum score
        # enforced by DefaultRecoveryConfig in the recovery engine.
        RESULT=$("${SEMANTIC}" find "$QUERY" --snapshot "$AFTER_FILE" --format json --threshold 0.52 2>/dev/null || echo '{"matches":[]}')
        BEST_REF=$(echo "$RESULT" | jq -r '.best_ref // ""')

        rm -f "$AFTER_FILE"

        RECOVERY_TOTAL=$((RECOVERY_TOTAL + 1))
        STATUS="FAIL"

        if [[ "$EXPECT_NO_MATCH" == "true" ]]; then
            if [[ -z "$BEST_REF" ]] || [[ "$BEST_REF" == "null" ]]; then
                STATUS="PASS"
                RECOVERY_SUCCESS=$((RECOVERY_SUCCESS + 1))
            fi
        elif [[ "$BEST_REF" == "$EXPECTED" ]]; then
            STATUS="PASS"
            RECOVERY_SUCCESS=$((RECOVERY_SUCCESS + 1))
        elif [[ -n "$EXPECTED_ALT" ]] && echo ",$EXPECTED_ALT," | grep -q ",$BEST_REF,"; then
            STATUS="PASS"
            RECOVERY_SUCCESS=$((RECOVERY_SUCCESS + 1))
        fi

        printf "  [%s] %s | %s | got=%s want=%s\n" "$ID" "$STATUS" "$NAME" "$BEST_REF" "$EXPECTED"
    done
fi

RECOVERY_RATE=0
if [[ $RECOVERY_TOTAL -gt 0 ]]; then
    RECOVERY_RATE=$(echo "scale=4; $RECOVERY_SUCCESS / $RECOVERY_TOTAL" | bc)
fi

echo ""
echo "  Recovery: $RECOVERY_SUCCESS / $RECOVERY_TOTAL = $RECOVERY_RATE"

echo ""
echo "=============================================="
echo "  PHASE 3: CLASSIFICATION BENCHMARK"
echo "=============================================="

CLASS_FILE="${CORPUS_DIR}/classification/cases.json"
CLASS_TOTAL=0
CLASS_CORRECT=0

if [[ -f "$CLASS_FILE" ]]; then
    CLASS_COUNT=$(jq length "$CLASS_FILE")

    for i in $(seq 0 $((CLASS_COUNT - 1))); do
        ID=$(jq -r ".[$i].id" "$CLASS_FILE")
        ERROR=$(jq -r ".[$i].error" "$CLASS_FILE")
        EXPECTED=$(jq -r ".[$i].expected_type" "$CLASS_FILE")

        # Run semantic classify (extract just the type, first word)
        RESULT=$("${SEMANTIC}" classify "$ERROR" 2>/dev/null || echo "unknown")
        GOT=$(echo "$RESULT" | awk '{print $1}')

        CLASS_TOTAL=$((CLASS_TOTAL + 1))
        STATUS="FAIL"

        if [[ "$GOT" == "$EXPECTED" ]]; then
            STATUS="PASS"
            CLASS_CORRECT=$((CLASS_CORRECT + 1))
        fi

        printf "  [%s] %s | \"%s\" → %s (want %s)\n" "$ID" "$STATUS" "${ERROR:0:40}" "$GOT" "$EXPECTED"
    done
fi

CLASS_ACCURACY=0
if [[ $CLASS_TOTAL -gt 0 ]]; then
    CLASS_ACCURACY=$(echo "scale=4; $CLASS_CORRECT / $CLASS_TOTAL" | bc)
fi

echo ""
echo "  Classification: $CLASS_CORRECT / $CLASS_TOTAL = $CLASS_ACCURACY"

echo ""
echo "=============================================="
echo "  COMPOSITE SCORE"
echo "=============================================="

# Calculate composite score with weights:
#   Find P@1:      40%
#   Find MRR:      20%
#   Recovery Rate: 25%
#   Classification: 15%

COMPOSITE=$(echo "scale=4; \
    ($FIND_P1 * 0.40) + \
    ($FIND_MRR * 0.20) + \
    ($RECOVERY_RATE * 0.25) + \
    ($CLASS_ACCURACY * 0.15)" | bc)

# Assign grade
GRADE="F"
if (( $(echo "$COMPOSITE >= 0.95" | bc -l) )); then GRADE="A+"
elif (( $(echo "$COMPOSITE >= 0.90" | bc -l) )); then GRADE="A"
elif (( $(echo "$COMPOSITE >= 0.85" | bc -l) )); then GRADE="B+"
elif (( $(echo "$COMPOSITE >= 0.80" | bc -l) )); then GRADE="B"
elif (( $(echo "$COMPOSITE >= 0.75" | bc -l) )); then GRADE="C+"
elif (( $(echo "$COMPOSITE >= 0.70" | bc -l) )); then GRADE="C"
elif (( $(echo "$COMPOSITE >= 0.60" | bc -l) )); then GRADE="D"
fi

# Update report
TMP=$(mktemp)
jq \
    --argjson find_total "${FIND_TOTAL:-0}" \
    --argjson find_mrr "${FIND_MRR:-0}" \
    --argjson find_p1 "${FIND_P1:-0}" \
    --argjson find_lat "${FIND_LAT:-0}" \
    --argjson rec_total "$RECOVERY_TOTAL" \
    --argjson rec_success "$RECOVERY_SUCCESS" \
    --argjson rec_rate "$RECOVERY_RATE" \
    --argjson class_total "$CLASS_TOTAL" \
    --argjson class_correct "$CLASS_CORRECT" \
    --argjson class_acc "$CLASS_ACCURACY" \
    --argjson composite "$COMPOSITE" \
    --arg grade "$GRADE" \
    '.find = { total: $find_total, mrr: $find_mrr, p_at_1: $find_p1, latency_p50: $find_lat } |
     .recovery = { total: $rec_total, recovered: $rec_success, rate: $rec_rate } |
     .classification = { total: $class_total, correct: $class_correct, accuracy: $class_acc } |
     .composite = { score: $composite, grade: $grade }' \
    "$REPORT_FILE" > "$TMP"
mv "$TMP" "$REPORT_FILE"

# Generate summary
SUMMARY_FILE="${REPORT_FILE%.json}_summary.md"
cat > "$SUMMARY_FILE" << EOF
# Semantic Benchmark Report

## Composite Score: ${COMPOSITE} (${GRADE})

| Component | Weight | Score | Weighted |
|-----------|--------|-------|----------|
| Find P@1 | 40% | ${FIND_P1:-0} | $(echo "scale=3; ${FIND_P1:-0} * 0.40" | bc) |
| Find MRR | 20% | ${FIND_MRR:-0} | $(echo "scale=3; ${FIND_MRR:-0} * 0.20" | bc) |
| Recovery | 25% | ${RECOVERY_RATE} | $(echo "scale=3; ${RECOVERY_RATE} * 0.25" | bc) |
| Classification | 15% | ${CLASS_ACCURACY} | $(echo "scale=3; ${CLASS_ACCURACY} * 0.15" | bc) |

## Find Performance
- Queries: ${FIND_TOTAL:-0}
- MRR: ${FIND_MRR:-0}
- P@1: ${FIND_P1:-0}
- Latency P50: ${FIND_LAT:-0} ms

## Recovery Performance
- Scenarios: ${RECOVERY_TOTAL}
- Recovered: ${RECOVERY_SUCCESS}
- Rate: ${RECOVERY_RATE}

## Classification Performance
- Cases: ${CLASS_TOTAL}
- Correct: ${CLASS_CORRECT}
- Accuracy: ${CLASS_ACCURACY}

## Grade Scale
| Grade | Score |
|-------|-------|
| A+ | >= 0.95 |
| A | >= 0.90 |
| B+ | >= 0.85 |
| B | >= 0.80 |
| C+ | >= 0.75 |
| C | >= 0.70 |
| D | >= 0.60 |
| F | < 0.60 |
EOF

# Cleanup
rm -f "${BENCHMARK_DIR}/semantic"

echo ""
echo "  ┌─────────────────────────────────────────┐"
echo "  │  COMPOSITE SCORE: ${COMPOSITE}  GRADE: ${GRADE}      │"
echo "  ├─────────────────────────────────────────┤"
echo "  │  Find P@1:       ${FIND_P1:-0}  (40%)            │"
echo "  │  Find MRR:       ${FIND_MRR:-0}  (20%)            │"
echo "  │  Recovery:       ${RECOVERY_RATE}  (25%)            │"
echo "  │  Classification: ${CLASS_ACCURACY}  (15%)            │"
echo "  └─────────────────────────────────────────┘"
echo ""
echo "Report: ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
