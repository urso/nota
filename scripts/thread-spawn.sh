#!/bin/bash
# Create a child thread linked to a parent.
# Thin wrapper around `nota thread spawn`.
# Usage: thread-spawn.sh <parent-id> <message>
#        thread-spawn.sh --human <parent-id> <message>
#
# This script is the agent-facing entry point, so the initial comment is
# recorded with author "agent" by default. Pass --human to record it under
# the git user instead.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

author_flag=(--agent)
args=()
for arg in "$@"; do
  if [ "$arg" = "--human" ]; then
    author_flag=()
  else
    args+=("$arg")
  fi
done

exec bash "$SCRIPT_DIR/nota.sh" thread spawn "${author_flag[@]}" "${args[@]}"
