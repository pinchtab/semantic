#!/bin/bash
source /e2e/lib.sh

echo "  ── Match: Single Element Scoring ──"

SNAPSHOT="/testdata/snapshots/login-page.json"

# Match existing ref
result=$(semantic match "sign in" e4 --snapshot "$SNAPSHOT")
assert_contains "$result" "ref=e4" "match: returns correct ref"
assert_contains "$result" "score=" "match: has score"
assert_contains "$result" "confidence=" "match: has confidence"

# Match with low relevance
result=$(semantic match "shopping cart" e4 --snapshot "$SNAPSHOT")
assert_contains "$result" "ref=e4" "match: low relevance still returns ref"

# Non-existent ref
set +e
result=$(semantic match "sign in" e99 --snapshot "$SNAPSHOT" 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "1" "match: non-existent ref exits with code 1"
assert_contains "$result" "not found" "match: error message for missing ref"

summary "match-single"
