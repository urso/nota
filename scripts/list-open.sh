#!/bin/bash
# Lists .nota/*.md files that have status: open in frontmatter.
# Resolves .nota/ from the git repo root (or by walking up from $PWD).
# Outputs absolute paths. Exits 0 even if none found.

root=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -z "$root" ]; then
  # Walk up from $PWD looking for a .nota/ directory
  d=$(pwd)
  while [ "$d" != "/" ]; do
    if [ -d "$d/.nota" ]; then
      root="$d"
      break
    fi
    d=$(dirname "$d")
  done
  # Fall back to $PWD if nothing found (will exit cleanly if no .nota/)
  root="${root:-$(pwd)}"
fi

dir="$root/.nota"

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
