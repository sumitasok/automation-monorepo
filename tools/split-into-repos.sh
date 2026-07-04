#!/usr/bin/env bash
# Promote framework/ and packs/shared/ into standalone git repos and re-mount
# them as submodules of this workspace. See docs/SHARING.md.
#
# Usage:
#   tools/split-into-repos.sh <framework-remote> <shared-remote>
# Example:
#   tools/split-into-repos.sh \
#     git@github.com:you/automation-framework.git \
#     git@github.com:you/automation-pack-shared.git
#
# By default this DRY-RUNS (prints commands). Set APPLY=1 to execute.
set -euo pipefail

FRAMEWORK_REMOTE="${1:?framework remote url required}"
SHARED_REMOTE="${2:?shared-pack remote url required}"
WS="$(cd "$(dirname "$0")/.." && pwd)"
run() { if [ "${APPLY:-0}" = "1" ]; then eval "$@"; else echo "+ $*"; fi; }

echo "# workspace: $WS"
echo "# 1) framework -> $FRAMEWORK_REMOTE"
run "cd '$WS/framework' && git init -q && git add -A && git commit -q -m 'framework' \
     && git branch -M main && git remote add origin '$FRAMEWORK_REMOTE' && git push -u origin main"

echo "# 2) shared pack -> $SHARED_REMOTE"
run "cd '$WS/packs/shared' && git init -q && git add -A && git commit -q -m 'shared pack' \
     && git branch -M main && git remote add origin '$SHARED_REMOTE' && git push -u origin main"

echo "# 3) re-mount as submodules in the workspace"
run "cd '$WS' && rm -rf framework packs/shared"
run "cd '$WS' && git submodule add '$FRAMEWORK_REMOTE' framework"
run "cd '$WS' && git submodule add '$SHARED_REMOTE' packs/shared"
run "cd '$WS' && git add -A && git commit -q -m 'mount framework + shared pack as submodules'"

echo
[ "${APPLY:-0}" = "1" ] && echo "done." || echo "dry-run only. re-run with APPLY=1 to execute."
