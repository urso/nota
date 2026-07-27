#!/bin/bash
# Mark a thread as resolved.
# Thin wrapper around `nota thread resolve`.
# Usage: thread-resolve.sh <thread-id>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" thread resolve "$@"
