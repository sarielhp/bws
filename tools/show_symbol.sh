#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <symbol-name>"
  echo "Show the declaration and surrounding context for a Go symbol."
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

symbol="$1"

# Search for the symbol across all .go files, showing file/line and context
find . -name '*.go' -not -path './.git/*' -not -path '*/vendor/*' -exec grep -rn "$symbol" {} + \
  | grep -E '(^|type |func |const |var )' \
  | sort -t: -k1,1 -k2,2n