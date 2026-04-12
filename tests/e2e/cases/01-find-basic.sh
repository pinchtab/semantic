#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Basic Queries ──"

# Login page
result=$(semantic find "sign in button" --snapshot "${ASSETS_DIR}/snapshots/login-page.json" --format json)
assert_json_field "$result" ".best_ref" "e4" "login: sign in button → e4"
assert_json_gte "$result" ".best_score" "0.5" "login: sign in score >= 0.5"

result=$(semantic find "email input" --snapshot ${ASSETS_DIR}/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e1" "login: email input → e1"

result=$(semantic find "password field" --snapshot ${ASSETS_DIR}/snapshots/login-page.json --format json)
assert_json_field "$result" ".best_ref" "e2" "login: password field → e2"

# Ecommerce
result=$(semantic find "add to cart" --snapshot ${ASSETS_DIR}/snapshots/ecommerce-product.json --format json)
assert_json_field "$result" ".best_ref" "e10" "ecommerce: add to cart → e10"
assert_json_gte "$result" ".best_score" "0.5" "ecommerce: add to cart score >= 0.5"

result=$(semantic find "quantity" --snapshot ${ASSETS_DIR}/snapshots/ecommerce-product.json --format json)
assert_json_field "$result" ".best_ref" "e8" "ecommerce: quantity → e8"

# Dashboard
result=$(semantic find "search box" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e6" "dashboard: search box → e6"

result=$(semantic find "log out" --snapshot ${ASSETS_DIR}/snapshots/dashboard.json --format json)
assert_json_field "$result" ".best_ref" "e15" "dashboard: log out → e15"

# Search results
result=$(semantic find "next page" --snapshot ${ASSETS_DIR}/snapshots/google-search.json --format json)
assert_json_field "$result" ".best_ref" "e9" "search: next page → e9"

summary "find-basic"
