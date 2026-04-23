#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  -- Find: Ordinal Queries --"

MULTI="${ASSETS_DIR}/snapshots/multi-form.json"

result=$(./semantic find "second submit button" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e7" "ordinal: second submit button → e7"

result=$(./semantic find "last submit button" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e11" "ordinal: last submit button → e11"

result=$(./semantic find "second submit button not in header" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e7" "ordinal+context: second submit button not in header → e7"

LOGIN="${ASSETS_DIR}/snapshots/login-page.json"
result=$(./semantic find "email address" --snapshot "$LOGIN" --format json)
assert_json_field "$result" ".best_ref" "e1" "guard: literal query still resolves email address → e1"

summary "find-ordinal"
