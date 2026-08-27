#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION_FILE="main.go"

# Read current version from main.go
current=$(grep -oP 'Version\s*=\s*"\K[^"]+' "$VERSION_FILE")
if [ -z "$current" ]; then
  echo "Error: cannot find Version in $VERSION_FILE"
  exit 1
fi

if [ $# -ge 1 ]; then
  new_version="$1"
  # Strip leading v if present
  new_version="${new_version#v}"
else
  # Auto-increment patch
  major="${current%%.*}"
  rest="${current#*.}"
  minor="${rest%%.*}"
  patch="${rest##*.}"
  patch=$((patch + 1))
  new_version="$major.$minor.$patch"
fi

echo "Current version: $current"
echo "New version:     $new_version"

# Rewrite Version line in main.go
sed -i "s/Version\s*=\s*\"$current\"/Version = \"$new_version\"/" "$VERSION_FILE"

git add "$VERSION_FILE"
git commit -m "chore: bump version to $new_version"
git push

echo "Done."