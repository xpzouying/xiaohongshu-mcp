#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

required_go="$(awk '/^go / {print $2; exit}' go.mod)"
required_go_major_minor="$(echo "$required_go" | cut -d. -f1,2)"
current_go="$(go version | awk '{print $3}' | sed 's/^go//')"
current_go_major_minor="$(echo "$current_go" | cut -d. -f1,2)"

if [[ "$(printf '%s\n' "$required_go_major_minor" "$current_go_major_minor" | sort -V | head -n1)" != "$required_go_major_minor" ]]; then
  echo "Go version check failed: required >= ${required_go_major_minor}, current = ${current_go_major_minor}"
  echo "Please upgrade Go before running full verification."
  exit 2
fi

echo "[1/3] gofmt"
GO_FILES="$(git ls-files '*.go')"
if [[ -n "$GO_FILES" ]]; then
  gofmt -w $GO_FILES
fi

echo "[2/3] go test ./..."
go test ./...

echo "[3/3] done"
