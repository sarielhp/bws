#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== go fmt ==="
gofumpt -l -w . 2>/dev/null || go fmt ./...

echo "=== go vet ==="
go vet ./...

unset LD_LIBRARY_PATH 2>/dev/null || true

echo "=== go build ==="
go build -o bws .

echo "=== go test ==="
go test ./... -count=1

echo ""
echo "All checks passed."