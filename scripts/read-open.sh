#!/bin/bash
# Shows a thread by ID.
# Thin wrapper around `nota thread show`.
# Usage: read-open.sh <thread-id>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" thread show "$@"
