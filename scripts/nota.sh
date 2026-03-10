#!/bin/bash
# Wrapper for the nota tool.
# Resolves repo root for --dir flag and ensures correct paths.
# Usage: nota.sh <subcommand> [flags] [files...]

set -euo pipefail

# Resolve plugin root from script location if not set by Claude Code
CLAUDE_PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

if ! command -v go &>/dev/null; then
  echo "error: Go is not installed. nota plugin requires Go." >&2
  exit 1
fi

root=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -z "$root" ]; then
  echo "error: not in a git repository" >&2
  exit 1
fi

subcommand="${1:-list}"
shift

# Build the binary in the plugin root, then run it from the current directory
# so that git-based file resolution works correctly.
bin="${CLAUDE_PLUGIN_ROOT}/.bin/nota"
needs_build=false
if [ ! -f "$bin" ]; then
  needs_build=true
elif [ -n "$(find "$CLAUDE_PLUGIN_ROOT" -name '*.go' -newer "$bin" -print -quit 2>/dev/null)" ] || [ "$CLAUDE_PLUGIN_ROOT/go.mod" -nt "$bin" ]; then
  needs_build=true
fi
if [ "$needs_build" = true ]; then
  mkdir -p "${CLAUDE_PLUGIN_ROOT}/.bin"
  tmp=$(mktemp "${bin}.XXXXXX")
  if go build -C "$CLAUDE_PLUGIN_ROOT" -o "$tmp" ./cmd/nota/; then
    mv "$tmp" "$bin"
  else
    rm -f "$tmp"
    exit 1
  fi
fi

case "$subcommand" in
  extract)
    exec "$bin" extract --dir "$root/.nota" "$@"
    ;;
  list|delete)
    exec "$bin" "$subcommand" "$@"
    ;;
  behavior)
    exec "$bin" behavior "$@"
    ;;
  *)
    echo "error: unknown subcommand: $subcommand" >&2
    exit 1
    ;;
esac
