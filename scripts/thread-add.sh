#!/bin/bash
# Add a comment to a thread.
# Thin wrapper around `nota thread add`.
# Usage: thread-add.sh <thread-id> <message>
#        thread-add.sh <thread-id> --file=- < message.md
#        thread-add.sh <thread-id> --local <message>
#        thread-add.sh --human <thread-id> <message>
#
# This script is the agent-facing entry point, so comments are recorded with
# author "agent" by default. Pass --human to record them under the git user
# instead (i.e. when relaying a comment the developer dictated).

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

exec bash "$SCRIPT_DIR/nota.sh" thread add "${author_flag[@]}" "${args[@]}"
