#!/bin/bash
#
# Update baseline after reviewing regressions.
#
# Usage:
#   ./update-baseline.sh --accept [--baseline <file>]
#
# This re-runs the benchmark and overwrites the baseline file.
# Use after reviewing check-baseline.sh output and confirming
# the changes are intentional.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
BASELINES_DIR="${BENCHMARK_DIR}/baselines"
CONFIG_FILE="${BENCHMARK_DIR}/config/benchmark.json"

# Read config
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

STRATEGY=$(jq -r '.defaults.strategy // "combined"' "$CONFIG_FILE")

# Parse args
BASELINE_FILE="${BASELINES_DIR}/${STRATEGY}.json"
ACCEPT=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --accept) ACCEPT=true; shift ;;
        --baseline) BASELINE_FILE="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ "$ACCEPT" != "true" ]]; then
    echo "Usage: $0 --accept [--baseline <file>]"
    echo ""
    echo "This will overwrite the baseline. Run check-baseline.sh first"
    echo "to review changes before accepting."
    exit 1
fi

if [[ ! -f "$BASELINE_FILE" ]]; then
    echo "Baseline not found: $BASELINE_FILE"
    echo "Creating new baseline instead..."
    exec "${SCRIPT_DIR}/create-baseline.sh" --name "$(basename "${BASELINE_FILE%.json}")"
fi

# Show what will change
echo "Current baseline: ${BASELINE_FILE}"
echo ""
jq -r '"  MRR:   \(.metrics.mrr)\n  P@1:   \(.metrics.p_at_1)\n  Hit@3: \(.metrics.hit_at_3)"' "$BASELINE_FILE"
echo ""
echo "Running benchmark to generate new baseline..."
echo ""

# Backup old baseline
BACKUP_FILE="${BASELINE_FILE%.json}_$(date +%Y%m%d_%H%M%S).backup.json"
cp "$BASELINE_FILE" "$BACKUP_FILE"
echo "Backed up old baseline to: $BACKUP_FILE"

# Create new baseline (overwrites)
"${SCRIPT_DIR}/create-baseline.sh" --name "$(basename "${BASELINE_FILE%.json}")"

echo ""
echo "Baseline updated. Old baseline backed up to:"
echo "  $BACKUP_FILE"
