#!/bin/bash
# Lists .aireview/*.md files that have status: open in frontmatter.
# Resolves .aireview/ from the git repo root and outputs absolute paths.
# Exits 0 even if none found.

root=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -z "$root" ]; then
  echo "error: not in a git repository" >&2
  exit 1
fi

dir="$root/.aireview"

if [ ! -d "$dir" ]; then
  exit 0
fi

for f in "$dir"/*.md; do
  [ -f "$f" ] || continue
  if awk '
    /^---$/ { count++; next }
    count == 1 && /^status:[[:space:]]*open/ { found=1; exit }
    count >= 2 { exit }
    END { exit !found }
  ' "$f"; then
    echo "$f"
  fi
done
