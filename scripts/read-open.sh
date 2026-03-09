#!/bin/bash
# Reads a tracking file and strips resolved/wontfix sections.
# A resolved section starts with ## [resolved] or ## [wontfix] and ends
# at the next ## heading or EOF.
# Usage: read-open.sh <file>

if [ -z "$1" ] || [ ! -f "$1" ]; then
  echo "Usage: read-open.sh <file>" >&2
  exit 1
fi

awk '
  /^## \[(resolved|wontfix)\]/ { skip=1; next }
  /^## / { skip=0 }
  !skip { print }
' "$1"
