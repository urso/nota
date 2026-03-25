#!/bin/bash
# Lists .nota/*.md files that have status: open in frontmatter.
# Thin wrapper around `nota find --status open`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" find --status open "$@"
