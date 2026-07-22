#!/bin/bash
# Query .nota/ threads by ID, status, tag, references, or dependencies.
# Thin wrapper around `nota thread list`. Delegates to the Go binary via nota.sh.
#
# Usage:
#   find-review.sh <id>                   Find thread by ID
#   find-review.sh --status open          List threads with given status
#   find-review.sh --tag security         List threads containing a given tag
#   find-review.sh --refs-of <id>         List threads that <id> references
#   find-review.sh --deps-of <id>         List threads that <id> depends on
#   find-review.sh --referenced-by <id>   List threads that reference <id>
#   find-review.sh --blocked-by <id>      List threads that depend on <id>
#
# Options can be combined. All filters are ANDed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# If first arg doesn't start with --, it's a thread ID lookup
if [ $# -gt 0 ] && [[ ! "$1" =~ ^-- ]]; then
  exec bash "$SCRIPT_DIR/nota.sh" thread show "$1"
fi

exec bash "$SCRIPT_DIR/nota.sh" thread list "$@"
