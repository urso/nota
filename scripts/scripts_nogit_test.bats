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

# Helper to create an XML thread file
# Usage: create_thread <filename> <id> <number> <status> [goal] [body]
create_thread() {
  local filename="$1" id="$2" number="$3" status="$4" goal="${5:-review}" body="${6:-Fix this}"
  cat > "$tmpdir/.nota/$filename" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="$id" number="$number" status="$status" goal="$goal">
  <nota-comment id="l:0000000000000001" author="user">
    <nota-body time="2026-01-01T00:00:00Z">$body</nota-body>
  </nota-comment>
</nota-thread>
EOF
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
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
}

@test "list-open (no git): walks up to find .nota in parent" {
  mkdir -p "$tmpdir/.nota"
  mkdir -p "$tmpdir/src/pkg"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir/src/pkg' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
}

@test "list-open (no git): resolved file not listed" {
  mkdir -p "$tmpdir/.nota"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open (no git): mixed open and resolved" {
  mkdir -p "$tmpdir/.nota"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
  [[ "$output" != *"l:efgh123456789012"* ]]
}

@test "list-open (no git): empty .nota directory" {
  mkdir -p "$tmpdir/.nota"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open (no git): multiple open files" {
  mkdir -p "$tmpdir/.nota"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "open"
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
  # We test with the 'local extract' subcommand and --all on a dir with no source files.
  # It should succeed (no files to scan = no output) rather than erroring about git.
  run bash -c "cd '$tmpdir' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' local extract --all 2>&1"
  [ "$status" -eq 0 ]
}

@test "nota.sh (no git): resolves root walking up from subdirectory" {
  mkdir -p "$tmpdir/.nota"
  mkdir -p "$tmpdir/src/deep/nested"
  run bash -c "cd '$tmpdir/src/deep/nested' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' local extract --all 2>&1"
  [ "$status" -eq 0 ]
}

# ============================================================
# find-review.sh — non-git fallback
# ============================================================

@test "find-review (no git): no .nota directory exits cleanly" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review (no git): finds thread by number" {
  mkdir -p "$tmpdir/.nota"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix this issue"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix this issue"* ]]
}

@test "find-review (no git): walks up to find .nota in parent" {
  mkdir -p "$tmpdir/.nota"
  mkdir -p "$tmpdir/src/pkg"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix this issue"
  run bash -c "cd '$tmpdir/src/pkg' && bash '$SCRIPT_DIR/find-review.sh' 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix this issue"* ]]
}

@test "find-review (no git): filter by status" {
  mkdir -p "$tmpdir/.nota"
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
  [[ "$output" != *"l:efgh123456789012"* ]]
}

@test "find-review (no git): deps-of follows depends-on" {
  mkdir -p "$tmpdir/.nota"
  # Create dependency thread
  cat > "$tmpdir/.nota/001-dep1234567890123.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="l:dep1234567890123" number="1" status="open" goal="review">
  <nota-comment id="l:0000000000000001" author="user">
    <nota-body time="2026-01-01T00:00:00Z">Fix first</nota-body>
  </nota-comment>
</nota-thread>
EOF
  # Create main thread that depends on it
  cat > "$tmpdir/.nota/002-main234567890123.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="l:main234567890123" number="2" status="open" goal="review" depends-on="l:dep1234567890123">
  <nota-comment id="l:0000000000000002" author="user">
    <nota-body time="2026-01-01T00:00:00Z">Fix after</nota-body>
  </nota-comment>
</nota-thread>
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of l:main234567890123"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:dep1234567890123"* ]]
}

# ============================================================
# nota.sh — extract
# ============================================================

@test "nota.sh (no git): extract finds comments in explicit files" {
  mkdir -p "$tmpdir/.nota"
  cat > "$tmpdir/test.go" <<'EOF'
package main

// REVIEW: check this logic
func foo() {}
EOF
  run bash -c "cd '$tmpdir' && CLAUDE_PLUGIN_ROOT='$SCRIPT_DIR/..' bash '$SCRIPT_DIR/nota.sh' local extract --dir '$tmpdir/.nota' '$tmpdir/test.go' 2>&1"
  [ "$status" -eq 0 ]
}
