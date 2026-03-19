#!/usr/bin/env bats

# Tests for shell scripts outside a git repository.

SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)"

setup() {
  tmpdir=$(cd "$(mktemp -d)" && pwd -P)
  # No git init — these tests verify non-git fallback behavior.
}

teardown() {
  rm -rf "$tmpdir"
}

# ============================================================
# list-open.sh — non-git fallback
# ============================================================

@test "list-open (no git): no .nota directory exits cleanly" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open (no git): finds open file via .nota in PWD" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/auth.md" ]
}

@test "list-open (no git): walks up to find .nota in parent" {
  mkdir -p "$tmpdir/.nota"
  mkdir -p "$tmpdir/src/pkg"
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
---

## review — src/auth/login.go:42

Fix
EOF
  run bash -c "cd '$tmpdir/src/pkg' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/auth.md" ]
}

@test "list-open (no git): resolved file not listed" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/auth/login.go:42

Fixed
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open (no git): mixed open and resolved" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
---

## review — src/auth/login.go:42

Fix
EOF
  cat > "$tmpdir/.nota/api.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/api/handler.go:10

Done
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/auth.md" ]
}

@test "list-open (no git): empty .nota directory" {
  mkdir -p "$tmpdir/.nota"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open (no git): multiple open files" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
---

## review — src/auth/login.go:42

Fix
EOF
  cat > "$tmpdir/.nota/api.md" <<'EOF'
---
status: open
---

## review — src/api/handler.go:10

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
}

# ============================================================
# nota.sh — non-git root resolution
# ============================================================

@test "nota.sh (no git): resolves root from .nota in PWD" {
  mkdir -p "$tmpdir/.nota"
  # nota.sh needs Go and builds a binary, so we just test that root resolution
  # works by checking that it gets past the root-finding stage.
  # We test with the 'extract' subcommand and --all on a dir with no source files.
  # It should succeed (no files to scan = no output) rather than erroring about git.
  run bash -c "cd '$tmpdir' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' extract --all 2>&1"
  [ "$status" -eq 0 ]
}

@test "nota.sh (no git): resolves root walking up from subdirectory" {
  mkdir -p "$tmpdir/.nota"
  mkdir -p "$tmpdir/src/deep/nested"
  run bash -c "cd '$tmpdir/src/deep/nested' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' extract --all 2>&1"
  [ "$status" -eq 0 ]
}

@test "nota.sh (no git): extract finds comments in explicit files" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/test.go" <<'EOF'
package main

// REVIEW: check this logic
func foo() {}
EOF
  run bash -c "cd '$tmpdir' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' extract --dir '$tmpdir/.nota' '$tmpdir/test.go' 2>&1"
  [ "$status" -eq 0 ]
}
