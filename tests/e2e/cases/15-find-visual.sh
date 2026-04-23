#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  -- Find: Visual Position Hints --"

VISUAL="${ASSETS_DIR}/snapshots/visual-layout.json"

result=$(semantic find "button in top right" --snapshot "$VISUAL" --format json)
assert_json_field "$result" ".best_ref" "e1" "visual: button in top right → e1 (Settings)"

result=$(semantic find "button on the left" --snapshot "$VISUAL" --format json)
assert_json_field "$result" ".best_ref" "e0" "visual: button on the left → e0 (Menu)"

result=$(semantic find "button at the bottom" --snapshot "$VISUAL" --format json)
assert_json_field "$result" ".best_ref" "e7" "visual: button at the bottom → e7 (Save)"

result=$(semantic find "link on left side" --snapshot "$VISUAL" --format json)
assert_json_field "$result" ".best_ref" "e3" "visual: link on left side → e3 (Help)"

summary "find-visual"
