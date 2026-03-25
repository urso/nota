#!/bin/bash
# Reads a tracking file and strips resolved/wontfix sections.
# Thin wrapper around `nota read`.
# Usage: read-open.sh <file>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" read "$@"
