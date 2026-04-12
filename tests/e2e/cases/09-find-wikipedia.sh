#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Find: Wikipedia Article Page ──"

SNAPSHOT="${ASSETS_DIR}/snapshots/wikipedia-article.json"

# Direct matches
result=$(semantic find "edit" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e6" "wikipedia: edit → e6"

result=$(semantic find "search wikipedia" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e0" "wikipedia: search wikipedia → e0"

result=$(semantic find "view history" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e7" "wikipedia: view history → e7"
assert_json_gte "$result" ".best_score" "0.5" "wikipedia: view history score >= 0.5"

result=$(semantic find "create account" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e25" "wikipedia: create account → e25"
assert_json_gte "$result" ".best_score" "0.4" "wikipedia: create account score >= 0.4"

result=$(semantic find "random article" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e4" "wikipedia: random article → e4"
assert_json_gte "$result" ".best_score" "0.5" "wikipedia: random article score >= 0.5"

# Synonym: get PDF → Download as PDF
result=$(semantic find "get PDF" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e21" "wikipedia: get PDF → download as PDF (e21)"

# Toggle sidebar
result=$(semantic find "toggle sidebar" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e23" "wikipedia: toggle sidebar → e23"

# Multiple headings
result=$(semantic find "heading" --snapshot "$SNAPSHOT" --format json --threshold 0.2 --top-k 10)
count=$(echo "$result" | jq '.matches | length')
assert_gte "$count" "2" "wikipedia: 'heading' returns multiple matches"

summary "find-wikipedia"
