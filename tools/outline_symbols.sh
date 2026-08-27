#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "=== Exported symbols ==="
echo ""

# Types, functions, constants, and vars across all packages
find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*' -exec grep -h '^type\s\+\w\+\s' {} + \
  | sed 's/^type \([[:alnum:]_]*\).*/type  \1/' \
  | sort -u

find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*' -exec grep -h '^func\s\+\w\+\s' {} + \
  | sed 's/^func \([[:alnum:]_]*\).*/func  \1/' \
  | sort -u

find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*' -exec grep -h '^const\s\+\w\+\s' {} + \
  | sed 's/^const \([[:alnum:]_]*\).*/const \1/' \
  | sort -u

find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*' -exec grep -h '^var\s\+\w\+\s' {} + \
  | sed 's/^var \([[:alnum:]_]*\).*/var   \1/' \
  | sort -u