#!/bin/bash
#
# Check for broken documentation links
#
# Usage:
#   ./scripts/check-docs-links.sh
#
set -uo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ERRORS=0

echo "Checking documentation links..."
echo ""

# Find all markdown files and check links
while IFS= read -r file; do
    dir=$(dirname "$file")

    # Extract markdown links: [text](path)
    while IFS= read -r link; do
        # Skip URLs and anchors
        if [[ "$link" =~ ^https?:// ]] || [[ "$link" =~ ^mailto: ]] || [[ "$link" =~ ^# ]]; then
            continue
        fi
        
        # Remove anchor from link
        link_path="${link%%#*}"
        
        # Skip empty paths
        if [[ -z "$link_path" ]]; then
            continue
        fi
        
        # Resolve relative path
        if [[ "$link_path" =~ ^/ ]]; then
            target="$link_path"
        else
            target="$dir/$link_path"
        fi
        
        # Check if target exists
        if [[ ! -e "$target" ]]; then
            echo -e "${RED}BROKEN:${NC} $file -> $link"
            ERRORS=$((ERRORS + 1))
        fi
    done < <(grep -oE '\]\([^)]+\)' "$file" 2>/dev/null | sed 's/\](//' | sed 's/)//')
done < <(find . -name "*.md" -not -path "./.git/*" -not -path "./node_modules/*")

echo ""
if [[ $ERRORS -eq 0 ]]; then
    echo -e "${GREEN}✓${NC} All documentation links valid"
    exit 0
else
    echo -e "${RED}Found $ERRORS broken link(s)${NC}"
    exit 1
fi
