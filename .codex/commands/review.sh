#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BASE="${1:-main}"

has_local_changes=false
if ! git diff --quiet || ! git diff --cached --quiet; then
  has_local_changes=true
fi
untracked_files="$(git ls-files --others --exclude-standard)"
if [[ -n "$untracked_files" ]]; then
  has_local_changes=true
fi

if [[ "$has_local_changes" == true ]]; then
  echo "Reviewing local working tree changes against HEAD"
  echo
  echo "== Changed Files =="
  git diff --name-only HEAD
  if [[ -n "$untracked_files" ]]; then
    echo "$untracked_files"
  fi
  echo
  echo "== Suspicious JS Injection Usage (Eval) =="
  git diff HEAD -- '*.go' | rg '^\+.*\.Eval\(' || true
  echo
  echo "== TODO/FIXME Added =="
  git diff HEAD | rg '^\+.*(TODO|FIXME)' || true
  echo
  echo "== Diff Stat =="
  git diff --stat HEAD
  if [[ -n "$untracked_files" ]]; then
    echo
    echo "== Untracked Files =="
    echo "$untracked_files"
  fi
  exit 0
fi

if git rev-parse --verify "$BASE" >/dev/null 2>&1; then
  diff_range="$BASE...HEAD"
else
  diff_range="HEAD~1...HEAD"
fi

echo "== Changed Files =="
git diff --name-only "$diff_range"

echo
echo "== Suspicious JS Injection Usage (Eval) =="
git diff "$diff_range" -- '*.go' | rg '^\+.*\.Eval\(' || true

echo
echo "== TODO/FIXME Added =="
git diff "$diff_range" | rg '^\+.*(TODO|FIXME)' || true

echo
echo "== Diff Stat =="
git diff --stat "$diff_range"
