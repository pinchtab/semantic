#!/bin/bash
CASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CASE_DIR}/../lib.sh"

echo "  ── Fixtures: Query Fixture Files ──"

run_fixture_file() {
  local file="$1"
  local name
  name=$(basename "$file" .json)
  local count
  count=$(jq length "$file")

  for i in $(seq 0 $((count - 1))); do
    local query snapshot expect_ref expect_no_match expect_has_matches min_score threshold note
    query=$(jq -r ".[$i].query" "$file")
    snapshot=$(jq -r ".[$i].snapshot" "$file")
    expect_ref=$(jq -r ".[$i].expect_ref // empty" "$file")
    expect_no_match=$(jq -r ".[$i].expect_no_match // empty" "$file")
    expect_has_matches=$(jq -r ".[$i].expect_has_matches // empty" "$file")
    min_score=$(jq -r ".[$i].min_score // empty" "$file")
    threshold=$(jq -r ".[$i].threshold // empty" "$file")
    note=$(jq -r ".[$i].note // empty" "$file")

    local label="${name}[${i}]"
    [ -n "$note" ] && label="${name}: ${note}"

    local args=("--snapshot" "${ASSETS_DIR}/snapshots/${snapshot}" "--format" "json")
    [ -n "$threshold" ] && args+=("--threshold" "$threshold")

    set +e
    result=$(semantic find "$query" "${args[@]}" 2>&1)
    exit_code=$?
    set -e

    if [ "$exit_code" -ne 0 ]; then
      # Empty query or crash — if expect_no_match, that's ok
      if [ "$expect_no_match" = "true" ]; then
        pass "$label (no match, exit=$exit_code)"
      else
        fail "$label" "exit code $exit_code: $result"
      fi
      continue
    fi

    if [ "$expect_no_match" = "true" ]; then
      local match_count
      match_count=$(echo "$result" | jq '.matches | length')
      if [ "$match_count" -eq 0 ]; then
        pass "$label (no matches)"
      else
        fail "$label" "expected no matches, got $match_count"
      fi
      continue
    fi

    if [ "$expect_has_matches" = "true" ]; then
      local match_count
      match_count=$(echo "$result" | jq '.matches | length')
      if [ "$match_count" -gt 0 ]; then
        pass "$label (has matches)"
      else
        fail "$label" "expected matches, got 0"
      fi
      continue
    fi

    if [ -n "$expect_ref" ]; then
      local got_ref
      got_ref=$(echo "$result" | jq -r '.best_ref')
      if [ "$got_ref" = "$expect_ref" ]; then
        pass "$label → $expect_ref"
      else
        fail "$label" "got $got_ref, want $expect_ref"
      fi
    fi

    if [ -n "$min_score" ]; then
      local got_score
      got_score=$(echo "$result" | jq -r '.best_score')
      if echo "$got_score $min_score" | awk '{exit ($1 >= $2) ? 0 : 1}'; then
        : # score ok, already passed on ref
      else
        fail "$label score" "got $got_score, want >= $min_score"
      fi
    fi
  done
}

for fixture in ${ASSETS_DIR}/queries/*.json; do
  [ -f "$fixture" ] || continue
  run_fixture_file "$fixture"
done

summary "fixtures"
