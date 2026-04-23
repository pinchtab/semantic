#!/bin/bash
#
# Run semantic matching benchmark with ranking metrics
#
# Usage:
#   ./run-corpus-benchmark.sh [--strategy <name>] [--corpus <dir>] [--lexical-weight <n>] [--embedding-weight <n>]
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
LEXICAL_WEIGHT=0.6
EMBEDDING_WEIGHT=0.4
while [[ $# -gt 0 ]]; do
    case "$1" in
        --strategy) STRATEGY="$2"; shift 2 ;;
        --corpus) SPECIFIC_CORPUS="$2"; shift 2 ;;
        --top-k) TOP_K="$2"; shift 2 ;;
        --lexical-weight) LEXICAL_WEIGHT="$2"; shift 2 ;;
        --embedding-weight) EMBEDDING_WEIGHT="$2"; shift 2 ;;
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
    --argjson lexical_weight "${LEXICAL_WEIGHT}" \
    --argjson embedding_weight "${EMBEDDING_WEIGHT}" \
    '{
        benchmark: {
            timestamp: $ts,
            strategy: $strategy,
            top_k: $top_k,
            type: "corpus",
            weights: {
                lexical: $lexical_weight,
                embedding: $embedding_weight
            }
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
declare -a ALL_HIT3=()
declare -a ALL_HIT5=()
declare -a ALL_MARGINS=()
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
            --lexical-weight "${LEXICAL_WEIGHT}" \
            --embedding-weight "${EMBEDDING_WEIGHT}" \
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
        local hit_at_3=0
        local hit_at_5=0
        local best_relevant_rank="null"
        for rank in 1 2 3 4 5; do
            local ref_at_rank
            ref_at_rank=$(echo "$result" | jq -r ".matches[$((rank-1))].ref // \"\"")
            if echo "$relevant_refs" | jq -e "index(\"${ref_at_rank}\")" > /dev/null 2>&1; then
                if [[ "$best_relevant_rank" == "null" ]]; then
                    best_relevant_rank=$rank
                fi
                if [[ $rank -le 3 ]]; then
                    relevant_in_top3=$((relevant_in_top3 + 1))
                    hit_at_3=1
                fi
                hit_at_5=1
            elif [[ $rank -le 3 ]]; then
                if echo "$partial_refs" | jq -e "index(\"${ref_at_rank}\")" > /dev/null 2>&1; then
                    partial_in_top3=$((partial_in_top3 + 1))
                fi
            fi
        done
        local p3
        p3=$(echo "scale=4; (${relevant_in_top3} + ${partial_in_top3} * 0.5) / 3" | bc)

        # Calculate best_relevant_score, best_wrong_score, and margin
        local best_relevant_score=0
        local best_wrong_score=0
        local num_matches
        num_matches=$(echo "$result" | jq '.matches | length')
        for idx in $(seq 0 $((num_matches - 1))); do
            local ref_at_idx score_at_idx
            ref_at_idx=$(echo "$result" | jq -r ".matches[$idx].ref // \"\"")
            score_at_idx=$(echo "$result" | jq -r ".matches[$idx].score // 0")
            if echo "$relevant_refs" | jq -e "index(\"${ref_at_idx}\")" > /dev/null 2>&1; then
                if (( $(echo "$score_at_idx > $best_relevant_score" | bc -l) )); then
                    best_relevant_score=$score_at_idx
                fi
            elif echo "$partial_refs" | jq -e "index(\"${ref_at_idx}\")" > /dev/null 2>&1; then
                : # partials don't count as wrong
            else
                if (( $(echo "$score_at_idx > $best_wrong_score" | bc -l) )); then
                    best_wrong_score=$score_at_idx
                fi
            fi
        done
        local margin
        margin=$(echo "scale=4; $best_relevant_score - $best_wrong_score" | bc)

        # Collect metrics
        ALL_RRS+=("$rr")
        ALL_P1+=("$p1")
        ALL_P3+=("$p3")
        ALL_HIT3+=("$hit_at_3")
        ALL_HIT5+=("$hit_at_5")
        ALL_MARGINS+=("$margin")
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
            --argjson hit_at_3 "$hit_at_3" \
            --argjson hit_at_5 "$hit_at_5" \
            --argjson best_relevant_rank "$best_relevant_rank" \
            --argjson best_relevant_score "$best_relevant_score" \
            --argjson best_wrong_score "$best_wrong_score" \
            --argjson margin "$margin" \
            --argjson latency "$duration_ms" \
            '{
                id: $id, query: $query, corpus: $corpus,
                difficulty: $difficulty, tags: $tags,
                best_ref: $best_ref, best_score: $best_score,
                matches: $matches, relevant_refs: $relevant,
                rr: $rr, p_at_1: $p1, p_at_3: $p3,
                hit_at_3: $hit_at_3, hit_at_5: $hit_at_5,
                best_relevant_rank: $best_relevant_rank,
                best_relevant_score: $best_relevant_score,
                best_wrong_score: $best_wrong_score,
                margin: $margin,
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

# Hit@3
HIT3=$(printf '%s\n' "${ALL_HIT3[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

# Hit@5
HIT5=$(printf '%s\n' "${ALL_HIT5[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

# Average margin
AVG_MARGIN=$(printf '%s\n' "${ALL_MARGINS[@]}" | awk '{s+=$1} END {printf "%.4f", s/NR}')

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
    --argjson hit3 "$HIT3" \
    --argjson hit5 "$HIT5" \
    --argjson avg_margin "$AVG_MARGIN" \
    --argjson lat_avg "$LAT_AVG" \
    --argjson lat_p50 "$LAT_P50" \
    --argjson lat_p95 "$LAT_P95" \
    --argjson lat_p99 "$LAT_P99" \
    '.metrics = {
        total: $total,
        mrr: $mrr,
        p_at_1: $p1,
        p_at_3: $p3,
        hit_at_3: $hit3,
        hit_at_5: $hit5,
        avg_margin: $avg_margin,
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
            p_at_1: ([.[].p_at_1] | add / length),
            hit_at_3: ([.[].hit_at_3] | add / length),
            hit_at_5: ([.[].hit_at_5] | add / length),
            avg_margin: ([.[].margin] | add / length)
        }
    }) | from_entries
)' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

# Add by-corpus breakdown
tmp=$(mktemp)
jq '.metrics.by_corpus = (
    .results | group_by(.corpus) | map({
        key: .[0].corpus,
        value: {
            count: length,
            mrr: ([.[].rr] | add / length),
            p_at_1: ([.[].p_at_1] | add / length),
            hit_at_3: ([.[].hit_at_3] | add / length),
            hit_at_5: ([.[].hit_at_5] | add / length),
            avg_margin: ([.[].margin] | add / length)
        }
    }) | from_entries
)' "$REPORT_FILE" > "$tmp"
mv "$tmp" "$REPORT_FILE"

# Add by-tag breakdown
tmp=$(mktemp)
jq '.metrics.by_tag = (
    [.results[] | {tags: .tags, rr: .rr, p_at_1: .p_at_1, hit_at_3: .hit_at_3, hit_at_5: .hit_at_5, margin: .margin}]
    | [.[] | .tags[] as $tag | {tag: $tag, rr: .rr, p_at_1: .p_at_1, hit_at_3: .hit_at_3, hit_at_5: .hit_at_5, margin: .margin}]
    | group_by(.tag)
    | map({
        key: .[0].tag,
        value: {
            count: length,
            mrr: ([.[].rr] | add / length),
            p_at_1: ([.[].p_at_1] | add / length),
            hit_at_3: ([.[].hit_at_3] | add / length),
            hit_at_5: ([.[].hit_at_5] | add / length),
            avg_margin: ([.[].margin] | add / length)
        }
    })
    | from_entries
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
| Lexical Weight | ${LEXICAL_WEIGHT} |
| Embedding Weight | ${EMBEDDING_WEIGHT} |
| Top-K | ${TOP_K} |
| Total Queries | ${TOTAL} |

## Ranking Metrics

| Metric | Value | Description |
|--------|-------|-------------|
| **MRR** | **${MRR}** | Mean Reciprocal Rank |
| **P@1** | **${P1}** | Precision at rank 1 |
| **P@3** | **${P3}** | Precision at rank 3 |
| **Hit@3** | **${HIT3}** | Any relevant in top 3 |
| **Hit@5** | **${HIT5}** | Any relevant in top 5 |
| **Avg Margin** | **${AVG_MARGIN}** | best_relevant - best_wrong |

## Latency

| Percentile | Value |
|------------|-------|
| Average | ${LAT_AVG} ms |
| P50 | ${LAT_P50} ms |
| P95 | ${LAT_P95} ms |
| P99 | ${LAT_P99} ms |

## By Difficulty

| Difficulty | Count | MRR | P@1 | Hit@3 | Margin |
|------------|-------|-----|-----|-------|--------|
$(jq -r '.metrics.by_difficulty | to_entries | .[] | "| \(.key) | \(.value.count) | \(.value.mrr | . * 100 | floor / 100) | \(.value.p_at_1 | . * 100 | floor / 100) | \(.value.hit_at_3 | . * 100 | floor / 100) | \(.value.avg_margin | . * 100 | floor / 100) |"' "$REPORT_FILE")

## By Corpus

| Corpus | Count | MRR | P@1 | Hit@3 | Margin |
|--------|-------|-----|-----|-------|--------|
$(jq -r '.metrics.by_corpus | to_entries | .[] | "| \(.key) | \(.value.count) | \(.value.mrr | . * 100 | floor / 100) | \(.value.p_at_1 | . * 100 | floor / 100) | \(.value.hit_at_3 | . * 100 | floor / 100) | \(.value.avg_margin | . * 100 | floor / 100) |"' "$REPORT_FILE")

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
echo "  Weights:     lexical=${LEXICAL_WEIGHT} embedding=${EMBEDDING_WEIGHT}"
echo "  Queries:     ${TOTAL}"
echo "  MRR:         ${MRR}"
echo "  P@1:         ${P1}"
echo "  P@3:         ${P3}"
echo "  Hit@3:       ${HIT3}"
echo "  Hit@5:       ${HIT5}"
echo "  Avg Margin:  ${AVG_MARGIN}"
echo "  Latency P50: ${LAT_P50} ms"
echo "  Latency P95: ${LAT_P95} ms"
echo "================================================"
echo ""
echo "Report:  ${REPORT_FILE}"
echo "Summary: ${SUMMARY_FILE}"
