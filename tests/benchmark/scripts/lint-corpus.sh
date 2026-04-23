#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="${SCRIPT_DIR}/.."
CORPUS_DIR="${BENCHMARK_DIR}/corpus"
CASES_DIR="${BENCHMARK_DIR}/cases"
SNAPSHOTS_DIR="${BENCHMARK_DIR}/../e2e/assets/snapshots"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

error() {
    echo -e "${RED}ERROR:${NC} $1"
    ((ERRORS++))
}

warn() {
    echo -e "${YELLOW}WARN:${NC} $1"
    ((WARNINGS++))
}

ok() {
    echo -e "${GREEN}✓${NC} $1"
}

echo "=== Corpus Lint ==="
echo ""

# 1. Check for invalid JSON in all benchmark files
echo "Checking JSON validity..."
for f in "${CORPUS_DIR}"/*/*.json "${CASES_DIR}"/*.json; do
    if [[ -f "$f" ]]; then
        if ! jq . "$f" >/dev/null 2>&1; then
            error "Invalid JSON: $f"
        fi
    fi
done

# 2. Check for duplicate query IDs across corpus files
echo "Checking for duplicate query IDs..."
declare -A QUERY_IDS
for f in "${CORPUS_DIR}"/*/queries.json; do
    if [[ -f "$f" ]]; then
        while IFS= read -r id; do
            if [[ -n "$id" && "$id" != "null" ]]; then
                if [[ -n "${QUERY_IDS[$id]:-}" ]]; then
                    error "Duplicate query ID '$id' in $f (first seen in ${QUERY_IDS[$id]})"
                else
                    QUERY_IDS[$id]="$f"
                fi
            fi
        done < <(jq -r '.[].id // empty' "$f" 2>/dev/null)
    fi
done

# Also check cases files
for f in "${CASES_DIR}"/*.json; do
    if [[ -f "$f" ]]; then
        while IFS= read -r id; do
            if [[ -n "$id" && "$id" != "null" ]]; then
                if [[ -n "${QUERY_IDS[$id]:-}" ]]; then
                    error "Duplicate query ID '$id' in $f (first seen in ${QUERY_IDS[$id]})"
                else
                    QUERY_IDS[$id]="$f"
                fi
            fi
        done < <(jq -r '.[].id // empty' "$f" 2>/dev/null)
    fi
done

# 3. Check for duplicate refs within snapshots
echo "Checking for duplicate refs in snapshots..."
for f in "${CORPUS_DIR}"/*/snapshot.json; do
    if [[ -f "$f" ]]; then
        dupes=$(jq -r '.[].ref' "$f" 2>/dev/null | sort | uniq -d)
        if [[ -n "$dupes" ]]; then
            error "Duplicate refs in $f: $dupes"
        fi
    fi
done

# 4. Check that relevant_refs exist in snapshot
echo "Checking relevant_refs exist in snapshots..."
for corpus_dir in "${CORPUS_DIR}"/*/; do
    corpus_name=$(basename "$corpus_dir")
    snapshot="${corpus_dir}snapshot.json"
    queries="${corpus_dir}queries.json"

    if [[ -f "$snapshot" && -f "$queries" ]]; then
        # Get all refs from snapshot
        refs=$(jq -r '.[].ref' "$snapshot" 2>/dev/null | sort | uniq)

        # Check relevant_refs
        while IFS= read -r ref; do
            if [[ -n "$ref" && "$ref" != "null" ]]; then
                if ! echo "$refs" | grep -qx "$ref"; then
                    error "[$corpus_name] relevant_ref '$ref' not found in snapshot"
                fi
            fi
        done < <(jq -r '.[].relevant_refs[]? // empty' "$queries" 2>/dev/null)

        # Check partially_relevant_refs
        while IFS= read -r ref; do
            if [[ -n "$ref" && "$ref" != "null" ]]; then
                if ! echo "$refs" | grep -qx "$ref"; then
                    error "[$corpus_name] partially_relevant_ref '$ref' not found in snapshot"
                fi
            fi
        done < <(jq -r '.[].partially_relevant_refs[]? // empty' "$queries" 2>/dev/null)
    fi
done

# 5. Check for empty relevant_refs (except no-match cases)
echo "Checking for empty relevant_refs..."
for f in "${CORPUS_DIR}"/*/queries.json; do
    if [[ -f "$f" ]]; then
        empty_relevant=$(jq -r '.[] | select(.relevant_refs | length == 0) | select(.partially_relevant_refs | length == 0) | select(.expect_no_match != true) | .id' "$f" 2>/dev/null)
        for id in $empty_relevant; do
            if [[ -n "$id" ]]; then
                warn "Query '$id' in $f has empty relevant_refs"
            fi
        done
    fi
done

# 6. Check difficulty values
echo "Checking difficulty values..."
VALID_DIFFICULTIES="easy medium hard"
for f in "${CORPUS_DIR}"/*/queries.json; do
    if [[ -f "$f" ]]; then
        while IFS= read -r line; do
            id=$(echo "$line" | cut -d'|' -f1)
            diff=$(echo "$line" | cut -d'|' -f2)
            if [[ -n "$diff" && "$diff" != "null" ]]; then
                if ! echo "$VALID_DIFFICULTIES" | grep -qw "$diff"; then
                    error "Invalid difficulty '$diff' for query '$id' in $f"
                fi
            fi
        done < <(jq -r '.[] | "\(.id)|\(.difficulty // "null")"' "$f" 2>/dev/null)
    fi
done

# 7. Check for known tags (warn on unknown)
echo "Checking tags..."
KNOWN_TAGS="absent-control accessibility action action-synonym action-verb adversarial alertdialog all-stopwords auth basket-cart bulk-action button cell checkbox combobox compound context-exclusion conversational dashboard description descriptive dialog directional disambiguation domain-intent download-export duplicate-labels ecommerce empty-query empty-snapshot exact exact-match filter find-search generic-verb github guard icon implicit input interactive-boost keyboard-mash legal link literal-text login login-signin long-query lookup-search media menu menuitem missing-letter name-match natural-language navigation negative-context no-match noise-tokens nonsense option ordinal pagination parent-context partial position preferences-settings purchase-buy question-form radio register-create registration repeated-word row-context search searchbox section section-context signout-logout single-char social special-chars spinbutton stale-ref state switch synonym synonym-chain tab table textbox threshold toggle transposition typo vague-query visual weak-match wikipedia"
for f in "${CORPUS_DIR}"/*/queries.json "${CASES_DIR}"/*.json; do
    if [[ -f "$f" ]]; then
        while IFS= read -r tag; do
            if [[ -n "$tag" && "$tag" != "null" ]]; then
                if ! echo "$KNOWN_TAGS" | grep -qw "$tag"; then
                    warn "Unknown tag '$tag' in $f"
                fi
            fi
        done < <(jq -r '.[].tags[]? // empty' "$f" 2>/dev/null)
    fi
done

# 8. Check case files reference existing snapshots
echo "Checking case file snapshot references..."
for f in "${CASES_DIR}"/*.json; do
    if [[ -f "$f" ]]; then
        while IFS= read -r snapshot; do
            if [[ -n "$snapshot" && "$snapshot" != "null" ]]; then
                if [[ ! -f "${SNAPSHOTS_DIR}/${snapshot}" ]]; then
                    error "Case file $f references missing snapshot: $snapshot"
                fi
            fi
        done < <(jq -r '.[].snapshot // empty' "$f" 2>/dev/null)
    fi
done

# 9. Check for generated result files in source tree
echo "Checking for generated result files..."
if ls "${BENCHMARK_DIR}"/results/*.json 2>/dev/null | grep -v '.gitkeep' | head -1 >/dev/null 2>&1; then
    result_count=$(ls "${BENCHMARK_DIR}"/results/*.json 2>/dev/null | wc -l | tr -d ' ')
    warn "Found $result_count generated result files in tests/benchmark/results/ (should be gitignored)"
fi

echo ""
echo "=== Summary ==="
if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
    ok "All checks passed"
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
    exit 0
else
    echo -e "${RED}Errors: $ERRORS${NC}"
    echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
    exit 1
fi
