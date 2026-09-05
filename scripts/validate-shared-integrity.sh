#!/bin/bash
# Validate shared/ directory integrity - ensure no unauthorized changes
# Usage: ./scripts/validate-shared-integrity.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "Shared Directory Integrity Validation"
echo "=========================================="
echo ""

ERRORS=0
WARNINGS=0

# Check that shared/ exists
if [ ! -d "$REPO_ROOT/packs/shared" ]; then
    echo -e "${RED}✗ ERROR: packs/shared/ not found${NC}"
    exit 1
fi

# Check for .lock file
if [ -f "$REPO_ROOT/packs/shared/.lock" ]; then
    echo -e "${GREEN}✓ Shared directory lock present${NC}"
else
    echo -e "${YELLOW}⚠ Shared directory lock missing${NC}"
    ((WARNINGS++))
fi

echo ""
echo "Checking required shared directories..."

required_dirs=(
    "auth"
    "jobs"
    "lib"
)

for dir in "${required_dirs[@]}"; do
    if [ -d "$REPO_ROOT/packs/shared/$dir" ]; then
        echo -e "${GREEN}✓ packs/shared/$dir exists${NC}"
    else
        echo -e "${RED}✗ packs/shared/$dir missing${NC}"
        ((ERRORS++))
    fi
done

echo ""
echo "Checking required shared files..."

required_files=(
    "packs/shared/package.json"
    "packs/shared/README.md"
)

for file in "${required_files[@]}"; do
    if [ -f "$REPO_ROOT/$file" ]; then
        echo -e "${GREEN}✓ $file exists${NC}"
    else
        echo -e "${YELLOW}⚠ $file missing${NC}"
        ((WARNINGS++))
    fi
done

echo ""
echo "Checking for unauthorized modifications..."

# Check git status of shared directory
cd "$REPO_ROOT"
MODIFIED=$(git status --porcelain packs/shared/ 2>/dev/null | grep -v "^??" | wc -l)

if [ "$MODIFIED" -gt 0 ]; then
    echo -e "${YELLOW}⚠ Modified files in packs/shared:${NC}"
    git status --porcelain packs/shared/ | grep -v "^??"
    ((WARNINGS++))
else
    echo -e "${GREEN}✓ No unauthorized modifications${NC}"
fi

echo ""
echo "=========================================="
echo "Validation Summary"
echo "=========================================="
echo -e "Errors:   ${RED}$ERRORS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo ""

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Shared directory integrity PASSED${NC}"
    exit 0
else
    echo -e "${RED}✗ Shared directory integrity FAILED${NC}"
    echo ""
    echo "Fix errors before proceeding:"
    echo "1. Ensure packs/shared/ has all required subdirectories"
    echo "2. Document any modifications in docs/adr/"
    echo "3. Get architecture team approval"
    exit 1
fi
