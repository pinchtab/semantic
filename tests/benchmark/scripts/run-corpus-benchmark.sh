#!/bin/bash
#
# Run semantic matching benchmark with ranking metrics
#
# Usage:
#   ./run-corpus-benchmark.sh [--strategy <name>] [--corpus <dir>]
#
# Metrics:
#   - MRR (Mean Reciprocal Rank)
#   - P@1 (Precision at 1)
#   - P@3 (Precision at 3)
#   - Latency distribution (p50, p95, p99)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CORPUS_DIR="${BENCHMARK_DIR}/corpus"
RESULTS_DIR="${BENCHMARK_DIR}/results"

# Parse args
STRATEGY="combined"
SPECIFIC_CORPUS=""
TOP_K=5
while [[ $# -gt 0 ]]; do
    case "$1" in
        --strategy) STRATEGY="$2"; shift 2 ;;
        --corpus) SPECIFIC_CORPUS="$2"; shift 2 ;;
        --top-k) TOP_K="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

case "${STRATEGY}" in
    lexical|embedding|combined) ;;
    *) echo "Unknown strategy: ${STRATEGY}"; exit 1 ;;
esac

mkdir -p "${RESULTS_DIR}"

# Build semantic binary
echo "Building semantic..."
(cd "${BENCHMARK_DIR}/../.." && go build -o "${BENCHMARK_DIR}/semantic" ./cmd/semantic)

SEMANTIC="${BENCHMARK_DIR}/semantic"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/corpus_${STRATEGY}_${TIMESTAMP}.json"

# Initialize report
jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg strategy "${STRATEGY}" \
    --argjson top_k "${TOP_K}" \
    '{
        benchmark: {
            timestamp: $ts,
            strategy: $strategy,
            top_k: $top_k,
            type: "corpus"
        },
        results: [],
        metrics: {
            total: 0,
            mrr: 0,
            p_at_1: 0,
            p_at_3: 0,
            latencies_ms: [],
            by_difficulty: {},
            by_tag: {}
        }
    }' > "${REPORT_FILE}"

# Arrays to collect metrics
declare -a ALL_RRS=()
declare -a ALL_P1=()
declare -a ALL_P3=()
declare -a ALL_LATENCIES=()

run_corpus() {
    local corpus_path="$1"
    local corpus_name
    corpus_name=$(basename "$corpus_path")

    local snapshot="${corpus_path}/snapshot.json"
    local queries="${corpus_path}/queries.json"

    if [[ ! -f "$snapshot" ]] || [[ ! -f "$queries" ]]; then
        if [[ -f "${corpus_path}/cases.json" ]] || [[ -f "${corpus_path}/scenarios.json" ]]; then
            return
        fi
        echo "  Skipping ${corpus_name}: missing files"
        return
    fi

    echo ""
    echo "=== Corpus: ${corpus_name} ==="

    local count
    count=$(jq length "$queries")

    for i in $(seq 0 $((count - 1))); do
        local id query relevant_refs partial_refs difficulty tags

        id=$(jq -r ".[$i].id" "$queries")
        query=$(jq -r ".[$i].query" "$queries")
        relevant_refs=$(jq -c ".[$i].relevant_refs" "$queries")
        partial_refs=$(jq -c ".[$i].partially_relevant_refs // []" "$queries")
        difficulty=$(jq -r ".[$i].difficulty // \"medium\"" "$queries")
        tags=$(jq -c ".[$i].tags // []" "$queries")

        # Run query and measure time
        local start_ns end_ns duration_ms result
        start_ns=$(python3 -c 'import time; print(int(time.time() * 1000000))')

        if ! result=$("${SEMANTIC}" find "${query}" \
            --snapshot "${snapshot}" \
            --strategy "${STRATEGY}" \
            --threshold 0.01 \
            --top-k "${TOP_K}" \
            --format json 2>&1); then
            echo "  [${id}] ERROR: semantic find failed for query: ${query}" >&2
            echo "${result}" >&2
            exit 1
        fi

        if ! echo "$result" | jq -e '(.matches | type) == "array"' > /dev/null 2>&1; then
            echo "  [${id}] ERROR: semantic find returned invalid JSON" >&2
            echo "${result}" >&2
            exit 1
        fi

        end_ns=$(python3 -c 'import time; print(int(time.time() * 1000000))')
        duration_ms=$(( (end_ns - start_ns) / 1000 ))

        # Extract results
        local matches best_ref best_score
        matches=$(echo "$result" | jq -c '[.matches[].ref]')
        best_ref=$(echo "$result" | jq -r '.best_ref // ""')
        best_score=$(echo "$result" | jq -r '.best_score // 0')

        # Calculate Reciprocal Rank
        local rr=0
        for rank in $(seq 1 ${TOP_K}); do
            local ref_at_rank
            ref_at_rank=$(echo "$result" | jq -r ".matches[$((rank-1))].ref // \"\"")
            if echo "$relevant_refs" | jq -e "index(\"${ref_at_rank}\")" > /dev/null 2>&1; then
                rr=$(echo "scale=4; 1 / ${rank}" | bc)
                break
            fi
        done

        # Calculate P@1
        local p1=0
        if echo "$relevant_refs" | jq -e "index(\"${best_ref}\")" > /dev/null 2>&1; then
            p1=1
        elif echo "$partial_refs" | jq -e "index(\"${best_ref}\")" > /dev/null 2>&1; then
            p1=0.5
        fi

        # Calculate P@3 (count relevant in top 3, partials count as 0.5)
        local relevant_in_top3=0
        local partial_in_top3=0
        for rank in 1 2 3; do
            local ref_at_rank
            ref_at_rank=$(echo "$result" | jq -r ".matches[$((rank-1))].ref // \"\"")
            if echo "$relevant_refs" | jq -e "index(\"${ref_at_rank}\")" > /dev/null 2>&1; then
                relevant_in_top3=$((relevant_in_top3 + 1))
            elif echo "$partial_refs" | jq -e "index(\"${ref_at_rank}\")" > /dev/null 2>&1; then
                partial_in_top3=$((partial_in_top3 + 1))
            fi
        done
        local p3
        p3=$(echo "scale=4; (${relevant_in_top3} + ${partial_in_top3} * 0.5) / 3" | bc)

        # Collect metrics
        ALL_RRS+=("$rr")
        ALL_P1+=("$p1")
        ALL_P3+=("$p3")
        ALL_LATENCIES+=("$duration_ms")

        # Status indicator
        local status="MISS"
        if (( $(echo "$p1 >= 1" | bc -l) )); then
            status="HIT "
        elif (( $(echo "$p1 >= 0.5" | bc -l) )); then
            status="PART"
        fi

        printf "  [%s] %s | RR=%.2f P@1=%.1f P@3=%.2f | %dms | %s\n" \
            "$id" "$status" "$rr" "$p1" "$p3" "$duration_ms" "$query"

        # Record to report
        local result_json
        result_json=$(jq -n \
            --arg id "$id" \
            --arg query "$query" \
            --arg corpus "$corpus_name" \
            --arg difficulty "$difficulty" \
            --argjson tags "$tags" \
            --arg best_ref "$best_ref" \
            --argjson best_score "$best_score" \
            --argjson matches "$matches" \
            --argjson relevant "$relevant_refs" \
            --argjson rr "$rr" \
            --argjson p1 "$p1" \
            --argjson p3 "$p3" \
            --argjson latency "$duration_ms" \
            '{
                id: $id, query: $query, corpus: $corpus,
                difficulty: $difficulty, tags: $tags,
                best_ref: $best_ref, best_score: $best_score,
                matches: $matches, relevant_refs: $relevant,
                rr: $rr, p_at_1: $p1, p_at_3: $p3,
                latency_ms: $latency
            }')

        # Append to report
        local tmp
        tmp=$(mktemp)
        jq --argjson r "$result_json" '.results += [$r]' "$REPORT_FILE" > "$tmp"
        mv "$tmp" "$REPORT_FILE"
    done
}

# Run benchmarks
if [[ -n "${SPECIFIC_CORPUS}" ]]; then
    run_corpus "${CORPUS_DIR}/${SPECIFIC_CORPUS}"
else
    for corpus in "${CORPUS_DIR}"/*/; do
        [[ -d "$corpus" ]] || continue
        run_corpus "$corpus"
    done
fi

# Calculate aggregate metrics
echo ""
echo "Calculating aggregate metrics..."

TOTAL=${#ALL_RRS[@]}
if [[ $TOTAL -eq 0 ]]; then
    echo "No results to aggregate"
    exit 1
fi

# MRR
MRR=$(printf '%s\n' "${ALL_RRS[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

# P@1
P1=$(printf '%s\n' "${ALL_P1[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

# P@3
P3=$(printf '%s\n' "${ALL_P3[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

# Latency percentiles
SORTED_LAT=($(printf '%s\n' "${ALL_LATENCIES[@]}" | sort -n))
P50_IDX=$(( TOTAL * 50 / 100 ))
P95_IDX=$(( TOTAL * 95 / 100 ))
P99_IDX=$(( TOTAL * 99 / 100 ))
LAT_P50=${SORTED_LAT[$P50_IDX]:-0}
LAT_P95=${SORTED_LAT[$P95_IDX]:-0}
LAT_P99=${SORTED_LAT[$P99_IDX]:-0}
LAT_AVG=$(printf '%s\n' "${ALL_LATENCIES[@]}" | awk '{s+=$1} END {printf "%.0f", s/NR}')

# Update report with aggregates
tmp=$(mktemp)
jq \
    --argjson total "$TOTAL" \
    --argjson mrr "$MRR" \
    --argjson p1 "$P1" \
    --argjson p3 "$P3" \
    --argjson lat_avg "$LAT_AVG" \
    --argjson lat_p50 "$LAT_P50" \
    --argjson lat_p95 "$LAT_P95" \
    --argjson lat_p99 "$LAT_P99" \
    '.metrics = {
        total: $total,
        mrr: $mrr,
        p_at_1: $p1,
        p_at_3: $p3,
        latency_avg_ms: $lat_avg,
        latency_p50_ms: $lat_p50,
        latency_p95_ms: $lat_p95,
        latency_p99_ms: $lat_p99
    }' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

# Add by-difficulty breakdown
tmp=$(mktemp)
jq '.metrics.by_difficulty = (
    .results | group_by(.difficulty) | map({
        key: .[0].difficulty,
        value: {
            count: length,
            mrr: ([.[].rr] | add / length),
            p_at_1: ([.[].p_at_1] | add / length)
        }
    }) | from_entries
)' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

# Generate summary
SUMMARY_FILE="${REPORT_FILE%.json}_summary.md"

cat > "${SUMMARY_FILE}" << EOF
# Semantic Matching Benchmark Results

## Configuration

| Field | Value |
|-------|-------|
| Timestamp | $(jq -r '.benchmark.timestamp' "$REPORT_FILE") |
| Strategy | ${STRATEGY} |
| Top-K | ${TOP_K} |
| Total Queries | ${TOTAL} |

## Ranking Metrics

| Metric | Value | Description |
|--------|-------|-------------|
| **MRR** | **${MRR}** | Mean Reciprocal Rank |
| **P@1** | **${P1}** | Precision at rank 1 |
| **P@3** | **${P3}** | Precision at rank 3 |

## Latency

| Percentile | Value |
|------------|-------|
| Average | ${LAT_AVG} ms |
| P50 | ${LAT_P50} ms |
| P95 | ${LAT_P95} ms |
| P99 | ${LAT_P99} ms |

## By Difficulty

$(jq -r '.metrics.by_difficulty | to_entries | .[] | "| \(.key) | \(.value.count) queries | MRR: \(.value.mrr | . * 100 | floor / 100) | P@1: \(.value.p_at_1 | . * 100 | floor / 100) |"' "$REPORT_FILE")

## Misses (P@1 = 0)

| ID | Query | Got | Expected |
|----|-------|-----|----------|
$(jq -r '.results[] | select(.p_at_1 == 0) | "| \(.id) | \(.query) | \(.best_ref) | \(.relevant_refs | join(",")) |"' "$REPORT_FILE")

EOF

# Cleanup
rm -f "${BENCHMARK_DIR}/semantic"

echo ""
echo "================================================"
echo "  CORPUS BENCHMARK RESULTS"
echo "================================================"
echo "  Strategy:    ${STRATEGY}"
echo "  Queries:     ${TOTAL}"
echo "  MRR:         ${MRR}"
echo "  P@1:         ${P1}"
echo "  P@3:         ${P3}"
echo "  Latency P50: ${LAT_P50} ms"
echo "  Latency P95: ${LAT_P95} ms"
echo "================================================"
echo ""
echo "Report:  ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
