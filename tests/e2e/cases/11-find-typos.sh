#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  -- Find: Typo Tolerance --"

SNAPSHOT="${ASSETS_DIR}/snapshots/login-page.json"

# sigin → sign in (transposition)
result=$(semantic find "sigin button" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e4" "typo: sigin → sign in (e4)"

# pasword → password (missing letter)
result=$(semantic find "pasword field" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e2" "typo: pasword → password (e2)"

# emial → email (transposition)
result=$(semantic find "emial input" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e1" "typo: emial → email (e1)"

# forgt → forgot (missing letter)
result=$(semantic find "forgt password" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e5" "typo: forgt → forgot (e5)"

# crate → create (missing letter)
result=$(semantic find "crate account" --snapshot "$SNAPSHOT" --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e6" "typo: crate → create (e6)"

# Ecommerce typos
ECOM="${ASSETS_DIR}/snapshots/ecommerce-product.json"

# ad to cart → add to cart (missing letter)
result=$(semantic find "ad to cart" --snapshot "$ECOM" --format json)
assert_json_field "$result" ".best_ref" "e10" "typo: ad to cart → add to cart (e10)"

# quantiy → quantity (transposition, needs low threshold)
result=$(semantic find "quantiy" --snapshot "$ECOM" --format json --threshold 0.1)
assert_json_field "$result" ".best_ref" "e8" "typo: quantiy → quantity (e8)"

# buton → button (missing letter)
result=$(semantic find "buton" --snapshot "$ECOM" --format json --threshold 0.1)
count=$(echo "$result" | jq '.matches | length')
assert_gte "$count" "1" "typo: buton matches at least one button"

# Dashboard typos
DASH="${ASSETS_DIR}/snapshots/dashboard.json"

# serch → search (missing letter)
result=$(semantic find "serch projects" --snapshot "$DASH" --format json)
assert_json_field "$result" ".best_ref" "e6" "typo: serch → search (e6)"

# exprot → export (transposition)
result=$(semantic find "exprot data" --snapshot "$DASH" --format json)
assert_json_field "$result" ".best_ref" "e8" "typo: exprot → export (e8)"

# lgo out → log out (transposition)
result=$(semantic find "lgo out" --snapshot "$DASH" --format json --threshold 0.2)
assert_json_field "$result" ".best_ref" "e15" "typo: lgo out → log out (e15)"

summary "find-typos"
