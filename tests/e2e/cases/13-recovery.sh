#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  -- Recovery: Confidence Threshold --"

# Recovery should accept medium/high confidence matches
# "log out" vs "Logout" - same meaning, different spelling
result=$(echo '[{"ref":"e5","role":"button","name":"Logout"}]' | semantic find "log out" --format json --threshold 0.5)
assert_json_field "$result" ".best_ref" "e5" "recovery: 'log out' matches 'Logout'"
assert_json_gte "$result" ".best_score" "0.5" "recovery: log out score >= 0.5"

# "sign in" vs "Login" - synonym phrase match (lower threshold due to embedding blend)
result=$(echo '[{"ref":"e1","role":"button","name":"Login"}]' | semantic find "sign in" --format json --threshold 0.4)
assert_json_field "$result" ".best_ref" "e1" "recovery: 'sign in' matches 'Login'"
assert_json_gte "$result" ".best_score" "0.4" "recovery: sign in score >= 0.4"

# Element renamed: "Submit" to "Send" - should still match
result=$(echo '[{"ref":"e3","role":"button","name":"Send"}]' | semantic find "submit button" --format json --threshold 0.5)
assert_json_field "$result" ".best_ref" "e3" "recovery: 'submit button' matches 'Send'"

# Element removed: "delete button" with no Delete - should NOT match Edit
result=$(echo '[{"ref":"e2","role":"button","name":"Edit"},{"ref":"e3","role":"button","name":"Archive"}]' | semantic find "delete button" --format json --threshold 0.52)
BEST_REF=$(echo "$result" | jq -r '.best_ref // ""')
BEST_SCORE=$(echo "$result" | jq -r '.best_score // 0')
# With threshold 0.52, Edit (0.50) should be filtered out
if [[ -z "$BEST_REF" ]] || [[ "$BEST_REF" == "null" ]]; then
    pass "recovery: 'delete button' correctly returns no match when element is removed"
else
    # If there's a match, it should only be Archive (not Edit)
    if [[ "$BEST_REF" == "e2" ]]; then
        fail "recovery: should not match Edit for 'delete button'" "got e2 (Edit)"
    else
        pass "recovery: 'delete button' returns non-Edit match: $BEST_REF"
    fi
fi

summary "recovery"
