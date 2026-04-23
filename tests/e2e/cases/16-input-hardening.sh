#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Input Hardening / Edge Cases ──"

SNAPSHOT="${ASSETS_DIR}/snapshots/login-page.json"

# Negative threshold (should be clamped to 0)
set +e
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --threshold -0.5 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "hardening: negative threshold doesn't crash"

# Threshold > 1 (should be clamped to 1, returning no matches)
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --threshold 1.5)
count=$(echo "$result" | jq '.matches | length')
assert_eq "$count" "0" "hardening: threshold > 1 returns no matches"

# Zero topk (should use default)
set +e
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --top-k 0 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "hardening: zero topk doesn't crash"

# Negative topk (should use default)
set +e
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --top-k -5 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "hardening: negative topk doesn't crash"

# Very large topk (should be clamped to element count)
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --top-k 10000 --threshold 0)
count=$(echo "$result" | jq '.matches | length')
elem_count=$(jq 'length' "$SNAPSHOT")
if [ "$count" -le "$elem_count" ]; then
  pass "hardening: large topk clamped to element count"
else
  fail "hardening: large topk clamped to element count" "got $count matches, expected <= $elem_count"
fi

# Custom weights that sum to > 1
set +e
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --lexical-weight 2 --embedding-weight 2 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "hardening: weights > 1 don't crash"

# Verify scores are still bounded [0,1] with extreme weights
best_score=$(echo "$result" | jq '.best_score')
if awk "BEGIN {exit !($best_score >= 0 && $best_score <= 1)}"; then
  pass "hardening: scores bounded with extreme weights"
else
  fail "hardening: scores bounded with extreme weights" "got score $best_score"
fi

summary "input-hardening"
