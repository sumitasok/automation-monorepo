#!/bin/bash
# Migration validation script: verify current flat pack structure → new domain structure
# Usage: ./scripts/validate-migration.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_PATH="${1:-$HOME/automation-monorepo-config}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Migration Validation Script"
echo "=========================================="
echo "Repo root: $REPO_ROOT"
echo "Config path: $CONFIG_PATH"
echo ""

ERRORS=0
WARNINGS=0
VALID_PACKS=()

# Check current pack structure
echo "Checking current pack structure..."
if [ ! -d "$REPO_ROOT/packs" ]; then
    echo -e "${RED}✗ ERROR: packs/ directory not found${NC}"
    exit 1
fi

# Identify current packs
CURRENT_PACKS=$(find "$REPO_ROOT/packs" -maxdepth 1 -type d ! -name "packs" ! -name "shared" ! -name "framework" | sort)

echo "Found packs:"
for pack in $CURRENT_PACKS; do
    pack_name=$(basename "$pack")
    echo "  - $pack_name"
    VALID_PACKS+=("$pack_name")
done
echo ""

# Verify required directories exist
echo "Validating required directories..."

required_dirs=(
    "packs/shared"
    "packs/framework"
)

for dir in "${required_dirs[@]}"; do
    if [ -d "$REPO_ROOT/$dir" ]; then
        echo -e "${GREEN}✓ $dir exists${NC}"
    else
        echo -e "${RED}✗ $dir missing${NC}"
        ((ERRORS++))
    fi
done
echo ""

# Check for shared files that must be preserved
echo "Checking shared framework files..."
shared_files=(
    "packs/shared/package.json"
    "packs/shared/lib"
    "packs/framework"
)

for file in "${shared_files[@]}"; do
    if [ -e "$REPO_ROOT/$file" ]; then
        echo -e "${GREEN}✓ $file exists${NC}"
    else
        echo -e "${YELLOW}⚠ $file missing (may be expected)${NC}"
        ((WARNINGS++))
    fi
done
echo ""

# Validate external config structure
echo "Validating external config structure at $CONFIG_PATH..."

required_config_dirs=(
    "$CONFIG_PATH/config"
    "$CONFIG_PATH/data"
    "$CONFIG_PATH/rules"
)

for dir in "${required_config_dirs[@]}"; do
    if [ -d "$dir" ]; then
        echo -e "${GREEN}✓ $(basename $dir)/ exists${NC}"
    else
        echo -e "${RED}✗ $(basename $dir)/ missing${NC}"
        ((ERRORS++))
    fi
done
echo ""

# Check for framework.yaml
if [ -f "$CONFIG_PATH/config/framework.yaml" ]; then
    echo -e "${GREEN}✓ framework.yaml exists${NC}"
else
    echo -e "${RED}✗ framework.yaml missing${NC}"
    ((ERRORS++))
fi
echo ""

# Validate pack-specific config structure (expense-domain example)
echo "Validating domain-specific config structure..."
if [ -d "$CONFIG_PATH/config/expense-domain" ]; then
    echo -e "${GREEN}✓ expense-domain config directory exists${NC}"
    if [ -f "$CONFIG_PATH/config/expense-domain/domain.yaml" ]; then
        echo -e "${GREEN}✓ expense-domain/domain.yaml exists${NC}"
    else
        echo -e "${YELLOW}⚠ expense-domain/domain.yaml missing (will be created)${NC}"
    fi
else
    echo -e "${YELLOW}⚠ expense-domain config directory missing (will be created)${NC}"
    ((WARNINGS++))
fi
echo ""

# Check .gitignore for external config exclusion
echo "Checking .gitignore..."
if grep -q "automation-monorepo-config" "$REPO_ROOT/.gitignore"; then
    echo -e "${GREEN}✓ automation-monorepo-config documented in .gitignore${NC}"
else
    echo -e "${YELLOW}⚠ automation-monorepo-config not mentioned in .gitignore${NC}"
    ((WARNINGS++))
fi
echo ""

# Summary
echo "=========================================="
echo "Validation Summary"
echo "=========================================="
echo -e "Errors:   ${RED}$ERRORS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo "Valid packs identified:"
for pack in "${VALID_PACKS[@]}"; do
    echo "  - $pack"
done
echo ""

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Migration structure validation PASSED${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Review domain structure in ARCHITECTURE.md"
    echo "2. Begin Phase 1 implementation tasks"
    echo "3. Use --config-path parameter when running framework"
    exit 0
else
    echo -e "${RED}✗ Migration structure validation FAILED${NC}"
    echo ""
    echo "Fix errors before proceeding:"
    echo "1. Ensure ~/automation-monorepo-config/ is created with config/, data/, rules/ subdirectories"
    echo "2. Create config/framework.yaml with framework settings"
    echo "3. Create domain-specific config directories"
    exit 1
fi
