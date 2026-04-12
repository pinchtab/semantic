#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Output Formats ──"

SNAPSHOT="${ASSETS_DIR}/snapshots/login-page.json"

# JSON format
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format json)
assert_contains "$result" '"best_ref"' "json: has best_ref field"
assert_contains "$result" '"matches"' "json: has matches array"
assert_contains "$result" '"confidence"' "json: has confidence field"

# Table format
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format table)
assert_contains "$result" "REF" "table: has REF header"
assert_contains "$result" "SCORE" "table: has SCORE header"
assert_contains "$result" "CONFIDENCE" "table: has CONFIDENCE header"

# Refs format
result=$(semantic find "sign in" --snapshot "$SNAPSHOT" --format refs)
assert_contains "$result" "e4" "refs: outputs ref e4"
# Refs should NOT have headers
if echo "$result" | grep -q "REF"; then
  fail "refs: should not have headers" "found REF header"
else
  pass "refs: no headers"
fi

# Stdin piping
result=$(cat "$SNAPSHOT" | semantic find "sign in" --format json)
assert_json_field "$result" ".best_ref" "e4" "stdin: piped snapshot works"

summary "find-formats"
