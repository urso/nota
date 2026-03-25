#!/bin/bash
# Query .nota/ tracking files by name, status, tag, references, or dependencies.
# Thin wrapper around `nota find`. Delegates to the Go binary via nota.sh.
#
# Usage:
#   find-review.sh <name>                 Find file by filename stem (e.g. "auth" → .nota/auth.md)
#   find-review.sh --status open          List files with given status
#   find-review.sh --tag security         List files containing a given tag
#   find-review.sh --refs-of <name>       List files that <name> references
#   find-review.sh --deps-of <name>       List files that <name> depends on
#   find-review.sh --referenced-by <name> List files that reference <name>
#   find-review.sh --blocked-by <name>    List files that depend on <name>
#
# Options can be combined. All filters are ANDed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" find "$@"
