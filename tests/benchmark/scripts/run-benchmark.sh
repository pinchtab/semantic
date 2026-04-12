#!/bin/bash
#
# Run semantic matching benchmark
#
# Usage:
#   ./run-benchmark.sh [--strategy <name>] [--cases <file>]
#
# Options:
#   --strategy <name>   Strategy to benchmark (lexical, embedding, combined, all)
#   --cases <file>      Specific case file to run (default: all)
#   --output <dir>      Output directory (default: ../results)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CASES_DIR="${BENCHMARK_DIR}/cases"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"
SNAPSHOTS_DIR="${BENCHMARK_DIR}/../e2e/assets/snapshots"
RESULTS_DIR="${BENCHMARK_DIR}/results"

# Parse args
STRATEGY="combined"
CASE_FILE=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --strategy) STRATEGY="$2"; shift 2 ;;
        --cases) CASE_FILE="$2"; shift 2 ;;
        --output) RESULTS_DIR="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

mkdir -p "${RESULTS_DIR}"

# Build semantic binary
echo "Building semantic..."
(cd "${BENCHMARK_DIR}/../.." && go build -o "${BENCHMARK_DIR}/semantic" ./cmd/semantic)

SEMANTIC="${BENCHMARK_DIR}/semantic"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/benchmark_${TIMESTAMP}.json"

# Initialize report
jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg strategy "${STRATEGY}" \
    --arg version "$(${SEMANTIC} --version 2>/dev/null || echo 'dev')" \
    '{
        benchmark: {
            timestamp: $ts,
            strategy: $strategy,
            version: $version
        },
        results: [],
        summary: {
            total: 0,
            passed: 0,
            failed: 0,
            skipped: 0,
            accuracy: 0,
            avg_score: 0,
            avg_latency_ms: 0
        }
    }' > "${REPORT_FILE}"

# Run cases
run_case() {
    local case_file="$1"
    local case_name
    case_name=$(basename "$case_file" .json)

    echo ""
    echo "=== Running: ${case_name} ==="

    local count
    count=$(jq length "$case_file")

    for i in $(seq 0 $((count - 1))); do
        local id query snapshot expect_ref expect_ref_alt expect_no_match expect_no_crash expect_has_matches threshold min_score

        id=$(jq -r ".[$i].id" "$case_file")
        query=$(jq -r ".[$i].query" "$case_file")
        snapshot=$(jq -r ".[$i].snapshot" "$case_file")
        expect_ref=$(jq -r ".[$i].expect_ref // empty" "$case_file")
        expect_ref_alt=$(jq -r ".[$i].expect_ref_alt // [] | join(\",\")" "$case_file")
        expect_no_match=$(jq -r ".[$i].expect_no_match // false" "$case_file")
        expect_no_crash=$(jq -r ".[$i].expect_no_crash // false" "$case_file")
        expect_has_matches=$(jq -r ".[$i].expect_has_matches // false" "$case_file")
        threshold=$(jq -r ".[$i].threshold // 0.3" "$case_file")
        min_score=$(jq -r ".[$i].min_score // 0" "$case_file")

        local snapshot_path="${SNAPSHOTS_DIR}/${snapshot}"
        if [[ ! -f "${snapshot_path}" ]]; then
            echo "  [${id}] SKIP: snapshot not found: ${snapshot}"
            "${SCRIPT_DIR}/record-result.sh" "${REPORT_FILE}" "${id}" "skip" 0 0 "snapshot not found"
            continue
        fi

        # Run query and measure time
        local start_ms end_ms duration_ms result exit_code
        start_ms=$(python3 -c 'import time; print(int(time.time() * 1000))')

        set +e
        result=$("${SEMANTIC}" find "${query}" \
            --snapshot "${snapshot_path}" \
            --strategy "${STRATEGY}" \
            --threshold "${threshold}" \
            --format json 2>&1)
        exit_code=$?
        set -e

        end_ms=$(python3 -c 'import time; print(int(time.time() * 1000))')
        duration_ms=$((end_ms - start_ms))

        # Evaluate result
        local status="fail"
        local got_ref=""
        local got_score=0
        local notes=""

        if [[ ${exit_code} -ne 0 ]]; then
            if [[ "${expect_no_crash}" == "true" ]]; then
                # Some crashes are expected (empty query, etc)
                status="pass"
                notes="exit ${exit_code} (expected)"
            else
                notes="exit ${exit_code}: ${result}"
            fi
        else
            got_ref=$(echo "$result" | jq -r '.best_ref // empty')
            got_score=$(echo "$result" | jq -r '.best_score // 0')
            local match_count
            match_count=$(echo "$result" | jq -r '.matches | length')

            if [[ "${expect_no_match}" == "true" ]]; then
                if [[ ${match_count} -eq 0 ]]; then
                    status="pass"
                    notes="no matches (expected)"
                else
                    notes="expected no matches, got ${match_count}"
                fi
            elif [[ "${expect_has_matches}" == "true" ]]; then
                if [[ ${match_count} -gt 0 ]]; then
                    status="pass"
                    notes="${match_count} matches"
                else
                    notes="expected matches, got 0"
                fi
            elif [[ -n "${expect_ref}" ]]; then
                if [[ "${got_ref}" == "${expect_ref}" ]]; then
                    status="pass"
                    notes="ref=${got_ref}, score=${got_score}"
                elif [[ -n "${expect_ref_alt}" ]] && echo ",${expect_ref_alt}," | grep -q ",${got_ref},"; then
                    status="pass"
                    notes="ref=${got_ref} (alt), score=${got_score}"
                else
                    notes="got ${got_ref}, want ${expect_ref}"
                fi
            elif [[ "${expect_no_crash}" == "true" ]]; then
                status="pass"
                notes="no crash"
            fi
        fi

        # Record result
        "${SCRIPT_DIR}/record-result.sh" "${REPORT_FILE}" "${id}" "${status}" "${got_score}" "${duration_ms}" "${notes}"

        if [[ "${status}" == "pass" ]]; then
            echo "  [${id}] PASS: ${notes}"
        else
            echo "  [${id}] FAIL: ${notes}"
        fi
    done
}

# Find case files
if [[ -n "${CASE_FILE}" ]]; then
    run_case "${CASES_DIR}/${CASE_FILE}"
else
    for case_file in "${CASES_DIR}"/*.json; do
        [[ -f "$case_file" ]] || continue
        run_case "$case_file"
    done
fi

# Finalize report
"${SCRIPT_DIR}/finalize-report.sh" "${REPORT_FILE}"

# Cleanup
rm -f "${BENCHMARK_DIR}/semantic"

echo ""
echo "Benchmark complete: ${REPORT_FILE}"
