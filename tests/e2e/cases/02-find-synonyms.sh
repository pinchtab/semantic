#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Synonym Resolution ──"

# log in → Sign In (sign in synonym)
result=$(semantic find "log in" --snapshot ${ASSETS_DIR}/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e4" "synonym: log in → sign in (e4)"
assert_json_gte "$result" ".best_score" "0.4" "synonym: log in score >= 0.4"

# register → Create an account (weak synonym, needs low threshold)
result=$(semantic find "register" --snapshot ${ASSETS_DIR}/snapshots/login-page.json --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e6" "synonym: register → create account (e6)"

# forgot password (direct match)
result=$(semantic find "forgot password" --snapshot ${ASSETS_DIR}/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e5" "synonym: forgot password → e5"
assert_json_gte "$result" ".best_score" "0.5" "synonym: forgot password score >= 0.5"

# purchase → Buy Now
result=$(semantic find "purchase" --snapshot ${ASSETS_DIR}/snapshots/ecommerce-product.json --format json)
assert_json_field "$result" ".best_ref" "e11" "synonym: purchase → buy now (e11)"

# basket → cart element (weak synonym, needs low threshold)
result=$(semantic find "basket" --snapshot ${ASSETS_DIR}/snapshots/ecommerce-product.json --format json --threshold 0.2)
best=$(echo "$result" | jq -r '.best_ref')
if [ "$best" = "e10" ] || [ "$best" = "e17" ]; then
  pass "synonym: basket → cart element ($best)"
else
  fail "synonym: basket → cart" "got $best, want e10 or e17"
fi

# preferences → Settings (weak synonym, needs low threshold)
result=$(semantic find "preferences" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e3" "synonym: preferences → settings (e3)"

# find projects → Search projects
result=$(semantic find "find projects" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e6" "synonym: find → search (e6)"

# download data → Export Data
result=$(semantic find "download data" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e8" "synonym: download → export (e8)"

# sign out → Log Out
result=$(semantic find "sign out" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e15" "synonym: sign out → log out (e15)"

# look up → Search (weak synonym, needs low threshold)
result=$(semantic find "look up" --snapshot ${ASSETS_DIR}/snapshots/google-search.json --format json --threshold 0.1)
best=$(echo "$result" | jq -r '.best_ref')
if [ "$best" = "e0" ] || [ "$best" = "e1" ]; then
  pass "synonym: look up → search element ($best)"
else
  fail "synonym: look up → search" "got $best, want e0 or e1"
fi

summary "find-synonyms"
