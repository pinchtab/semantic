#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

BOLD=$'\033[1m'
ACCENT=$'\033[38;2;251;191;36m'
SUCCESS=$'\033[38;2;0;229;204m'
ERROR=$'\033[38;2;230;57;70m'
NC=$'\033[0m'

echo ""
echo "  ${ACCENT}${BOLD}🐳 E2E Tests (Docker)${NC}"
echo ""

# Clean previous results
rm -f tests/e2e/results/summary*.txt

# Build and run
set +e
docker compose -f tests/e2e/docker-compose.yml up --build --abort-on-container-exit --exit-code-from runner
EXIT_CODE=$?
set -e

# Cleanup
docker compose -f tests/e2e/docker-compose.yml down -v 2>/dev/null || true

echo ""
if [ "$EXIT_CODE" -eq 0 ]; then
  echo "  ${SUCCESS}${BOLD}E2E tests passed${NC}"
else
  echo "  ${ERROR}${BOLD}E2E tests failed${NC}"
fi

exit "$EXIT_CODE"
