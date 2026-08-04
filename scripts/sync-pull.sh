#!/bin/bash
# Pull GitHub PR comments into nota threads.
# Thin wrapper around `nota sync pull`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" sync pull "$@"
