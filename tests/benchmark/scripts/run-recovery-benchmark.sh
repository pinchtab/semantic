#!/bin/bash
#
# Recovery Engine Benchmark
#
# Exercises RecoveryEngine directly using before/after snapshots
# and intent cache entries from recovery scenarios.
#
# Usage:
#   ./run-recovery-benchmark.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
RESULTS_DIR="${BENCHMARK_DIR}/results"

mkdir -p "${RESULTS_DIR}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${RESULTS_DIR}/recovery_benchmark_${TIMESTAMP}.txt"

echo "=== Recovery Engine Benchmark ==="
echo ""

cd "${BENCHMARK_DIR}/../.."

# Run the Go test that exercises RecoveryEngine with scenarios
echo "Running recovery scenarios..."
echo ""

go test -v -run TestRecoveryBenchmark_Scenarios ./recovery/ 2>&1 | tee "$REPORT_FILE"

# Also run the Go benchmark for performance
echo ""
echo "Running performance benchmark..."
go test -bench=BenchmarkRecoveryEngine_Scenarios -benchmem ./recovery/ 2>&1 | tee -a "$REPORT_FILE"

echo ""
echo "================================================"
echo "  RECOVERY BENCHMARK COMPLETE"
echo "================================================"
echo "Report: $REPORT_FILE"
