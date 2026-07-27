#!/bin/bash
# Extract comments to .nota/ tracking files.
# Usage: extract.sh [flags] [files...]

set -euo pipefail

CLAUDE_PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

root=$(git rev-parse --show-toplevel 2>/dev/null) || true
if [ -z "$root" ]; then
  d=$(pwd)
  while [ "$d" != "/" ]; do
    if [ -d "$d/.nota" ]; then
      root="$d"
      break
    fi
    d=$(dirname "$d")
  done
  if [ -z "$root" ]; then
    root=$(pwd)
  fi
fi

exec bash "$CLAUDE_PLUGIN_ROOT/scripts/nota.sh" local extract --dir "$root/.nota" "$@"
