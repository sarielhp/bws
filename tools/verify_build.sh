#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== go fmt ==="
gofumpt -l -w . 2>/dev/null || go fmt ./...

echo "=== go vet ==="
go vet ./...

echo "=== go test ==="
go test ./... -count=1

echo "=== go build ==="
go build -o bws .
ln -sf bws bw

echo ""
echo "All checks passed."