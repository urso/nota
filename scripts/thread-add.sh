#!/bin/bash
# Add a comment to a thread.
# Thin wrapper around `nota thread add`.
# Usage: thread-add.sh <thread-id> <message>
#        thread-add.sh <thread-id> --file=- < message.md
#        thread-add.sh <thread-id> --local <message>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" thread add "$@"
