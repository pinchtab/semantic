#!/bin/bash
source /e2e/lib.sh

echo "  -- Find: Context Disambiguation --"

MULTI="/testdata/snapshots/multi-form.json"

# Section disambiguation: multiple Submit buttons
result=$(semantic find "submit in login" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e3" "context: submit in login → e3 (Login section)"

result=$(semantic find "submit in payment" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e7" "context: submit in payment → e7 (Payment section)"

result=$(semantic find "submit in shipping" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e11" "context: submit in shipping → e11 (Shipping section)"

# Parent context
result=$(semantic find "login form email" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e1" "context: login form email → e1"

result=$(semantic find "payment card" --snapshot "$MULTI" --format json)
assert_json_field "$result" ".best_ref" "e4" "context: payment card → e4"

# Interactive element boosting for action queries
LOGIN="/testdata/snapshots/login-page.json"

# "click sign in" should prefer interactive button over non-interactive elements
result=$(semantic find "click sign in" --snapshot "$LOGIN" --format json)
assert_json_field "$result" ".best_ref" "e4" "interactive: click sign in → e4 (interactive button)"
assert_json_gte "$result" ".best_score" "0.4" "interactive: action query has good score"

# Dashboard context
DASH="/testdata/snapshots/dashboard.json"

# Sidebar vs Toolbar elements
result=$(semantic find "settings in sidebar" --snapshot "$DASH" --format json)
assert_json_field "$result" ".best_ref" "e3" "context: settings in sidebar → e3"

result=$(semantic find "toolbar export" --snapshot "$DASH" --format json)
assert_json_field "$result" ".best_ref" "e8" "context: toolbar export → e8"

# Header elements
result=$(semantic find "header notifications" --snapshot "$DASH" --format json)
assert_json_field "$result" ".best_ref" "e10" "context: header notifications → e10"

# Ecommerce context
ECOM="/testdata/snapshots/ecommerce-product.json"

# Product Actions section
result=$(semantic find "product actions add to cart" --snapshot "$ECOM" --format json)
assert_json_field "$result" ".best_ref" "e10" "context: product actions add to cart → e10"

# Tabs in Product Details
result=$(semantic find "product details reviews tab" --snapshot "$ECOM" --format json)
assert_json_field "$result" ".best_ref" "e15" "context: reviews tab in product details → e15"

summary "find-context"
