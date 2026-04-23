#!/bin/bash
# E2E test helper library

# Compute paths - support both container (/e2e, /testdata) and local development
if [ -d "/e2e" ]; then
  E2E_DIR="/e2e"
  ASSETS_DIR="/testdata"
else
  # Find lib.sh location and compute paths from there
  E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  ASSETS_DIR="${E2E_DIR}/assets"
fi

if [ "$E2E_DIR" != "/e2e" ] && [ -z "${SEMANTIC_E2E_BOOTSTRAPPED:-}" ]; then
  REPO_ROOT="$(cd "${E2E_DIR}/../.." && pwd)"
  if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: go is required to run local E2E tests" >&2
    exit 1
  fi
  if ! (cd "$REPO_ROOT" && go build -o "${E2E_DIR}/semantic" ./cmd/semantic); then
    echo "ERROR: failed to build semantic binary for local E2E tests" >&2
    exit 1
  fi
  export PATH="${E2E_DIR}:$PATH"
  export SEMANTIC_E2E_BOOTSTRAPPED=1
fi

PASSED=0
FAILED=0
ERRORS=""

pass() {
  PASSED=$((PASSED + 1))
  echo "  ✓ $1"
}

fail() {
  FAILED=$((FAILED + 1))
  ERRORS="${ERRORS}\n  ✗ $1: $2"
  echo "  ✗ $1: $2"
}

assert_eq() {
  local got="$1" want="$2" msg="$3"
  if [ "$got" = "$want" ]; then
    pass "$msg"
  else
    fail "$msg" "got '$got', want '$want'"
  fi
}

assert_not_empty() {
  local got="$1" msg="$2"
  if [ -n "$got" ]; then
    pass "$msg"
  else
    fail "$msg" "got empty string"
  fi
}

assert_gte() {
  local got="$1" want="$2" msg="$3"
  if echo "$got $want" | awk '{exit ($1 >= $2) ? 0 : 1}'; then
    pass "$msg"
  else
    fail "$msg" "got $got, want >= $want"
  fi
}

assert_exit_code() {
  local got="$1" want="$2" msg="$3"
  if [ "$got" -eq "$want" ]; then
    pass "$msg"
  else
    fail "$msg" "exit code $got, want $want"
  fi
}

assert_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if echo "$haystack" | grep -q "$needle"; then
    pass "$msg"
  else
    fail "$msg" "output does not contain '$needle'"
  fi
}

assert_json_field() {
  local json="$1" field="$2" want="$3" msg="$4"
  local got
  got=$(echo "$json" | jq -r "$field")
  if [ "$got" = "$want" ]; then
    pass "$msg"
  else
    fail "$msg" ".$field = '$got', want '$want'"
  fi
}

assert_json_gte() {
  local json="$1" field="$2" want="$3" msg="$4"
  local got
  got=$(echo "$json" | jq -r "$field")
  assert_gte "$got" "$want" "$msg"
}

summary() {
  local suite="$1"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  $suite"
  echo "  Passed: $PASSED"
  echo "  Failed: $FAILED"
  if [ -n "$ERRORS" ]; then
    echo ""
    echo -e "$ERRORS"
  fi
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  # Write results for CI
  if [ -d /results ]; then
    echo "passed=$PASSED" > "/results/summary-${suite}.txt"
    echo "failed=$FAILED" >> "/results/summary-${suite}.txt"
  fi
}
