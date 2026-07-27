#!/bin/bash
# Create a child thread linked to a parent.
# Thin wrapper around `nota thread spawn`.
# Usage: thread-spawn.sh <parent-id> <message>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" thread spawn "$@"
