#!/bin/bash
# Lists open threads from .nota/*.xml files.
# Thin wrapper around `nota thread list --status=open`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec bash "$SCRIPT_DIR/nota.sh" thread list --status=open "$@"
