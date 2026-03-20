#!/bin/bash
source /e2e/lib.sh

echo "  ── Find: GitHub Repo Page ──"

SNAPSHOT="/testdata/snapshots/github-repo.json"

# Direct matches
result=$(semantic find "star button" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e10" "github: star button → e10"
assert_json_gte "$result" ".best_score" "0.5" "github: star score >= 0.5"

result=$(semantic find "fork" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e9" "github: fork → e9"

result=$(semantic find "pull requests tab" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e4" "github: pull requests tab → e4"

result=$(semantic find "issues tab" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e3" "github: issues tab → e3"

result=$(semantic find "actions" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e5" "github: actions → e5"

result=$(semantic find "README" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e14" "github: README → e14"

result=$(semantic find "watch repository" --snapshot "$SNAPSHOT" --format json)
assert_json_field "$result" ".best_ref" "e8" "github: watch repository → e8"

# Verify multiple matches for generic query
result=$(semantic find "link" --snapshot "$SNAPSHOT" --format json --threshold 0.2 --top-k 10)
count=$(echo "$result" | jq '.matches | length')
assert_gte "$count" "3" "github: 'link' returns multiple matches"

summary "find-github"
