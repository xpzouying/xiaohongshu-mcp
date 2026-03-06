#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
BASE="${1:-main}"

echo "# PR: ${BRANCH}"
echo
echo "## Summary"
git diff --stat "${BASE}...HEAD" || true
echo
echo "## Key Changes"
git log --oneline "${BASE}..HEAD" || true
echo
echo "## Verification"
echo "- [ ] ./.codex/commands/verify.sh"
echo "- [ ] Manual smoke test"
