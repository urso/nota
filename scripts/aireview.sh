#!/bin/bash
# Wrapper for the aireview tool.
# Resolves repo root for --dir flag and ensures correct paths.
# Usage: aireview.sh <subcommand> [flags] [files...]

set -euo pipefail

if ! command -v go &>/dev/null; then
  echo "error: Go is not installed. aireview plugin requires Go." >&2
  exit 1
fi

root=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -z "$root" ]; then
  echo "error: not in a git repository" >&2
  exit 1
fi

subcommand="${1:-list}"
shift

case "$subcommand" in
  extract)
    exec go run "${CLAUDE_PLUGIN_ROOT}/cmd/aireview/" extract --dir "$root/.aireview" "$@"
    ;;
  list|delete)
    exec go run "${CLAUDE_PLUGIN_ROOT}/cmd/aireview/" "$subcommand" "$@"
    ;;
  *)
    echo "error: unknown subcommand: $subcommand" >&2
    exit 1
    ;;
esac
