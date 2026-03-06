#!/usr/bin/env bash
set -euo pipefail

TOPIC="${1:-task}"
DATE="$(date +%F)"
OUT="docs/plans/${DATE}-${TOPIC}-plan.md"

cat > "${OUT}" <<TPL
# ${TOPIC} Implementation Plan

## 1. Goal

## 2. Scope

## 3. Task Breakdown
- T1: input / output / verify / fail-signal
- T2: input / output / verify / fail-signal

## 4. Risks

## 5. Verification
- Command:
- Expected:
TPL

echo "Plan generated: ${OUT}"
