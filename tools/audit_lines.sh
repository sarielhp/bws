#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

soft=300
hard=500
has_error=false

echo "=== Auditing Go file line counts ==="

while IFS= read -r f; do
  lines=$(wc -l < "$f")
  if [ "$lines" -gt "$hard" ]; then
    echo "ERROR: $f has $lines lines (hard limit: $hard)"
    has_error=true
  elif [ "$lines" -gt "$soft" ]; then
    echo "WARNING: $f has $lines lines (soft limit: $soft)"
  fi
done < <(find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*')

if [ "$has_error" = true ]; then
  echo ""
  echo "FAIL: Some files exceed the $hard-line hard limit."
  exit 1
fi

echo "All files within limits."