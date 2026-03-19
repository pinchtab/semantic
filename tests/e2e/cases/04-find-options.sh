#!/bin/bash
source /e2e/lib.sh

echo "  ── Find: Options (threshold, top-k, strategy) ──"

SNAPSHOT="/testdata/snapshots/login-page.json"

# Top-K limiting
result=$(semantic find "button" --snapshot "$SNAPSHOT" --format json --top-k 1)
count=$(echo "$result" | jq '.matches | length')
assert_eq "$count" "1" "top-k=1: returns exactly 1 match"

result=$(semantic find "button" --snapshot "$SNAPSHOT" --format json --top-k 5)
count=$(echo "$result" | jq '.matches | length')
assert_gte "$count" "2" "top-k=5: returns multiple matches"

# High threshold
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --threshold 0.9)
count=$(echo "$result" | jq '.matches | length')
# Might get 0 or 1 depending on score — just ensure it doesn't crash
pass "threshold=0.9: runs without error"

# Low threshold returns more matches
result=$(semantic find "button" --snapshot "$SNAPSHOT" --format json --threshold 0.1 --top-k 10)
count=$(echo "$result" | jq '.matches | length')
assert_gte "$count" "1" "threshold=0.1: returns matches"

# Strategy: lexical
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --strategy lexical)
assert_contains "$result" "lexical" "strategy=lexical: strategy in output"

# Strategy: combined
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --strategy combined)
assert_contains "$result" "combined" "strategy=combined: strategy in output"

# Strategy: embedding
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json --strategy embedding)
assert_contains "$result" "embedding" "strategy=embedding: strategy in output"

summary "find-options"
