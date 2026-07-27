#!/usr/bin/env bats

SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)"

setup() {
  tmpdir=$(cd "$(mktemp -d)" && pwd -P)
  git init -q "$tmpdir"
  mkdir -p "$tmpdir/.nota"
}

teardown() {
  rm -rf "$tmpdir"
}

# Helper to create an XML thread file
# Usage: create_thread <filename> <id> <number> <status> [goal] [body] [extra_attrs]
create_thread() {
  local filename="$1" id="$2" number="$3" status="$4" goal="${5:-review}" body="${6:-Fix this}" extra="${7:-}"
  cat > "$tmpdir/.nota/$filename" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="$id" number="$number" status="$status" goal="$goal"$extra>
  <nota-comment id="l:0000000000000001" author="user">
    <nota-body time="2026-01-01T00:00:00Z">$body</nota-body>
  </nota-comment>
</nota-thread>
EOF
}

# Helper: run validate-tracking with a JSON input
run_validate() {
  local file="$1"
  echo "{\"tool_input\":{\"file_path\":\"$file\"}}" | bash "$SCRIPT_DIR/validate-tracking.sh"
}

# ============================================================
# list-open.sh
# ============================================================

@test "list-open: no .nota directory" {
  rmdir "$tmpdir/.nota"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: empty .nota directory" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: one open file" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
}

@test "list-open: resolved file not listed" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: mixed open and resolved" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
  [[ "$output" != *"l:efgh123456789012"* ]]
}

@test "list-open: multiple open files" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
}

# ============================================================
# read-open.sh (now thread show)
# ============================================================

@test "read-open: shows thread by ID" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix token expiry"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:abcd123456789012"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix token expiry"* ]]
}

@test "read-open: shows thread by number" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix token expiry"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix token expiry"* ]]
}

@test "read-open: missing thread exits non-zero" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:nonexistent1234567"
  [ "$status" -ne 0 ]
}

@test "read-open: no arguments exits non-zero" {
  run bash "$SCRIPT_DIR/read-open.sh"
  [ "$status" -ne 0 ]
}

# ============================================================
# validate-tracking.sh
# ============================================================

@test "validate: valid open file" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run run_validate "$tmpdir/.nota/001-abcd123456789012.xml"
  [ "$status" -eq 0 ]
}

@test "validate: valid resolved file" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "resolved"
  run run_validate "$tmpdir/.nota/001-abcd123456789012.xml"
  [ "$status" -eq 0 ]
}

@test "validate: invalid XML fails" {
  echo "not valid xml" > "$tmpdir/.nota/bad.xml"
  run run_validate "$tmpdir/.nota/bad.xml"
  [ "$status" -ne 0 ]
}

@test "validate: non-.nota file is skipped" {
  echo "random content" > "$tmpdir/random.xml"
  run run_validate "$tmpdir/random.xml"
  [ "$status" -eq 0 ]
}

@test "validate: invalid goal fails" {
  cat > "$tmpdir/.nota/bad.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="l:abcd123456789012" number="1" status="open" goal="invalid">
  <nota-comment id="l:0000000000000001" author="user">
    <nota-body time="2026-01-01T00:00:00Z">Fix</nota-body>
  </nota-comment>
</nota-thread>
EOF
  run run_validate "$tmpdir/.nota/bad.xml"
  [ "$status" -ne 0 ]
}

# ============================================================
# find-review.sh
# ============================================================

@test "find-review: lookup by number" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix this issue"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix this issue"* ]]
}

@test "find-review: lookup by ID" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review" "Fix this issue"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' l:abcd123456789012"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Fix this issue"* ]]
}

@test "find-review: ID not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' l:nonexistent1234567"
  [ "$status" -ne 0 ]
}

@test "find-review: filter by status" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "resolved"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:abcd123456789012"* ]]
  [[ "$output" != *"l:efgh123456789012"* ]]
}

@test "find-review: filter by goal" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open" "review"
  create_thread "002-efgh123456789012.xml" "l:efgh123456789012" 2 "open" "impl"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --goal impl"
  [ "$status" -eq 0 ]
  [[ "$output" != *"l:abcd123456789012"* ]]
  [[ "$output" == *"l:efgh123456789012"* ]]
}

@test "find-review: deps-of returns dependency files" {
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

@test "find-review: blocked-by returns files that depend on thread" {
  # Create blocker thread
  cat > "$tmpdir/.nota/001-blocker1234567890.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="l:blocker1234567890" number="1" status="open" goal="review">
  <nota-comment id="l:0000000000000001" author="user">
    <nota-body time="2026-01-01T00:00:00Z">Fix first</nota-body>
  </nota-comment>
</nota-thread>
EOF
  # Create blocked thread
  cat > "$tmpdir/.nota/002-blocked1234567890.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<nota-thread id="l:blocked1234567890" number="2" status="open" goal="review" depends-on="l:blocker1234567890">
  <nota-comment id="l:0000000000000002" author="user">
    <nota-body time="2026-01-01T00:00:00Z">Fix after</nota-body>
  </nota-comment>
</nota-thread>
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --blocked-by l:blocker1234567890"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:blocked1234567890"* ]]
}

@test "find-review: deps-of with no dependencies returns empty" {
  create_thread "001-standalone123456.xml" "l:standalone123456" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of l:standalone123456"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: deps-of nonexistent thread returns error" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of l:nonexistent1234567"
  [ "$status" -ne 0 ]
}

@test "find-review: empty .nota directory" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: no .nota directory exits cleanly" {
  rmdir "$tmpdir/.nota"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ============================================================
# thread-add.sh
# ============================================================

@test "thread-add: adds comment to thread by ID" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' l:abcd123456789012 'New comment'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Added comment"* ]]
  # Verify comment was added
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:abcd123456789012"
  [[ "$output" == *"New comment"* ]]
}

@test "thread-add: adds comment to thread by number" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' 1 'Comment via number'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Added comment"* ]]
}

# ============================================================
# thread-resolve.sh
# ============================================================

@test "thread-resolve: marks thread as resolved" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-resolve.sh' l:abcd123456789012"
  [ "$status" -eq 0 ]
  [[ "$output" == *"resolved"* ]]
  # Verify status changed
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status resolved"
  [[ "$output" == *"l:abcd123456789012"* ]]
}

@test "thread-resolve: works with number" {
  create_thread "001-abcd123456789012.xml" "l:abcd123456789012" 1 "open"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-resolve.sh' 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *"resolved"* ]]
}
