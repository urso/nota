#!/bin/bash
# PostToolUse hook: validates .nota/ tracking files after Edit.
# Receives hook input as JSON on stdin.
# Exits 0 if valid or not applicable, non-zero with error message if invalid.

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

if [ -z "$file" ] || [ ! -f "$file" ]; then
  exit 0
fi

# Only validate files in .nota/
case "$file" in
  .nota/*|*/.nota/*) ;;
  *) exit 0 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Delegate to nota validate via nota.sh.
output=$(bash "$SCRIPT_DIR/nota.sh" validate "$file" 2>&1) || {
  echo "$output"
  exit 1
}
