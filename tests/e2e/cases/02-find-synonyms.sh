#!/bin/bash
source /e2e/lib.sh

echo "  ── Find: Synonym Resolution ──"

# log in → Sign In (sign in synonym)
result=$(semantic find "log in" --snapshot /testdata/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e4" "synonym: log in → sign in (e4)"
assert_json_gte "$result" ".best_score" "0.4" "synonym: log in score >= 0.4"

# register → Create an account (needs lower threshold — synonym expands but score is modest)
result=$(semantic find "register" --snapshot /testdata/snapshots/login-page.json --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e6" "synonym: register → create account (e6)"

# forgot password (direct match)
result=$(semantic find "forgot password" --snapshot /testdata/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e5" "synonym: forgot password → e5"
assert_json_gte "$result" ".best_score" "0.5" "synonym: forgot password score >= 0.5"

# purchase → Buy Now
result=$(semantic find "purchase" --snapshot /testdata/snapshots/ecommerce-product.json --format json)
assert_json_field "$result" ".best_ref" "e11" "synonym: purchase → buy now (e11)"

# basket → View Cart (strongest cart synonym match)
result=$(semantic find "basket" --snapshot /testdata/snapshots/ecommerce-product.json --format json --threshold 0.2)
best=$(echo "$result" | jq -r '.best_ref')
# Either e10 (Add to Cart) or e17 (View Cart) is acceptable
if [ "$best" = "e10" ] || [ "$best" = "e17" ]; then
  pass "synonym: basket → cart element ($best)"
else
  fail "synonym: basket → cart" "got $best, want e10 or e17"
fi

# preferences → Settings (synonym match, needs lower threshold)
result=$(semantic find "preferences" --snapshot /testdata/snapshots/dashboard.json --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e3" "synonym: preferences → settings (e3)"

# find projects → Search projects
result=$(semantic find "find projects" --snapshot /testdata/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e6" "synonym: find → search (e6)"

# download data → Export Data
result=$(semantic find "download data" --snapshot /testdata/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e8" "synonym: download → export (e8)"

# sign out → Log Out
result=$(semantic find "sign out" --snapshot /testdata/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e15" "synonym: sign out → log out (e15)"

summary "find-synonyms"
