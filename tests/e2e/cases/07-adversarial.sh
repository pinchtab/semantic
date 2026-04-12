#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Adversarial / Edge Cases ──"

SNAPSHOT="${ASSETS_DIR}/snapshots/login-page.json"

# All-stopword query (should not crash)
set +e
result=$(semantic find "the the the the" --snapshot "$SNAPSHOT" --format json 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "adversarial: all-stopword query doesn't crash"

# Nonsense query
result=$(semantic find "xyzzy foobar baz" --snapshot "$SNAPSHOT" --format json --threshold 0.5)
count=$(echo "$result" | jq '.matches | length')
assert_eq "$count" "0" "adversarial: nonsense query returns no matches at threshold 0.5"

# Very long query
long_query="click the big blue sign in button on the top right of the login page please"
set +e
result=$(semantic find "$long_query" --snapshot "$SNAPSHOT" --format json 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "adversarial: long query doesn't crash"

# Empty snapshot
set +e
result=$(echo "[]" | semantic find "sign in" --format json 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "adversarial: empty snapshot doesn't crash"

# Single element snapshot
result=$(echo '[{"ref":"e0","role":"button","name":"OK"}]' | semantic find "ok button" --format json)
assert_json_field "$result" ".best_ref" "e0" "adversarial: single element matches"

# Special characters in query
set +e
result=$(semantic find "sign in (click here)" --snapshot "$SNAPSHOT" --format json 2>&1)
exit_code=$?
set -e
assert_eq "$exit_code" "0" "adversarial: parens in query don't crash"

summary "adversarial"
