#!/bin/bash
# PreToolUse hook: block direct deletion of .nota/ files.
# Deletion should only happen via /nota-cleanup after user confirmation.

set -euo pipefail

# Read the tool input from stdin (JSON)
input=$(cat)

tool_name=$(echo "$input" | jq -r '.tool_name // empty')
tool_input=$(echo "$input" | jq -r '.tool_input // empty')

# Only check Bash commands
if [ "$tool_name" != "Bash" ]; then
  exit 0
fi

command=$(echo "$tool_input" | jq -r '.command // empty')

# Check for rm commands targeting .nota/ paths
if echo "$command" | grep -qE '(rm|unlink)\s.*\.nota/'; then
  echo "BLOCKED: Direct deletion of .nota/ files is not allowed."
  echo "Use /nota-cleanup to remove resolved review files after confirmation."
  exit 2
fi

exit 0
