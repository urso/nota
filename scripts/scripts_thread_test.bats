#!/usr/bin/env bats

# Tests for thread-based scripts (XML format)

SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)"

setup() {
  tmpdir=$(cd "$(mktemp -d)" && pwd -P)
  git init -q "$tmpdir"
  mkdir -p "$tmpdir/.nota"
  # Configure git user for thread creation
  git -C "$tmpdir" config user.name "test"
  git -C "$tmpdir" config user.email "test@example.com"
}

teardown() {
  rm -rf "$tmpdir"
}

# Helper: create a thread XML file
write_thread() {
  local id="$1"
  local status="$2"
  local goal="$3"
  local message="$4"
  local group="${5:-}"
  local depends_on="${6:-}"

  local filename="${id#l:}.xml"
  local group_attr=""
  local depends_attr=""

  [ -n "$group" ] && group_attr="group=\"$group\""
  [ -n "$depends_on" ] && depends_attr="depends-on=\"$depends_on\""

  cat > "$tmpdir/.nota/$filename" <<EOF
<?xml version="1.0"?>
<nota-thread id="$id" status="$status" goal="$goal" $group_attr $depends_attr>
  <nota-comment id="l:c001" author="test">
    <nota-body time="2026-07-22T10:00:00Z">$message</nota-body>
  </nota-comment>
</nota-thread>
EOF
}

# Helper: create a thread with references
write_thread_with_refs() {
  local id="$1"
  local status="$2"
  local goal="$3"
  local message="$4"
  local ref_thread="$5"

  local filename="${id#l:}.xml"

  cat > "$tmpdir/.nota/$filename" <<EOF
<?xml version="1.0"?>
<nota-thread id="$id" status="$status" goal="$goal">
  <nota-ref thread="$ref_thread"/>
  <nota-comment id="l:c001" author="test">
    <nota-body time="2026-07-22T10:00:00Z">$message</nota-body>
  </nota-comment>
</nota-thread>
EOF
}

# ============================================================
# list-open.sh (thread version)
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

@test "list-open: one open thread" {
  write_thread "l:0001000100010001" "open" "review" "Fix token expiry"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:0001000100010001"* ]]
  [[ "$output" == *"open"* ]]
  [[ "$output" == *"review"* ]]
  [[ "$output" == *"Fix token expiry"* ]]
}

@test "list-open: resolved thread not listed" {
  write_thread "l:0001000100010001" "resolved" "review" "Fixed"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: mixed open and resolved" {
  write_thread "l:0001000100010001" "open" "review" "Fix this"
  write_thread "l:0002000200020002" "resolved" "review" "Already fixed"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:0001000100010001"* ]]
  [[ "$output" != *"l:0002000200020002"* ]]
}

@test "list-open: multiple open threads" {
  write_thread "l:0001000100010001" "open" "review" "Fix A"
  write_thread "l:0002000200020002" "open" "discuss" "Discuss B"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
}

@test "list-open: wontfix thread not listed" {
  write_thread "l:0001000100010001" "wontfix" "review" "Not fixing"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ============================================================
# read-open.sh (thread show)
# ============================================================

@test "read-open: shows thread content" {
  write_thread "l:0001000100010001" "open" "review" "Fix token expiry"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:0001000100010001"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Thread l:0001000100010001"* ]]
  [[ "$output" == *"Status: open"* ]]
  [[ "$output" == *"Goal: review"* ]]
  [[ "$output" == *"Fix token expiry"* ]]
}

@test "read-open: thread not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:nonexistent"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "read-open: no arguments exits non-zero" {
  run bash "$SCRIPT_DIR/read-open.sh"
  [ "$status" -ne 0 ]
}

@test "read-open: shows thread with anchor" {
  cat > "$tmpdir/.nota/anchored.xml" <<'EOF'
<?xml version="1.0"?>
<nota-thread id="l:anchor01" status="open" goal="review">
  <nota-anchor file="src/auth.go" line="42" commit="abc123"/>
  <nota-comment id="l:c001" author="test">
    <nota-body time="2026-07-22T10:00:00Z">Fix this line</nota-body>
  </nota-comment>
</nota-thread>
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:anchor01"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Anchor: src/auth.go:42"* ]]
  [[ "$output" == *"abc123"* ]]
}

# ============================================================
# find-review.sh (thread queries)
# ============================================================

@test "find-review: lookup by ID shows thread" {
  write_thread "l:0001000100010001" "open" "review" "Fix this"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' l:0001000100010001"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Thread l:0001000100010001"* ]]
}

@test "find-review: ID not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' l:nonexistent"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "find-review: filter by status" {
  write_thread "l:0001000100010001" "open" "review" "Open thread"
  write_thread "l:0002000200020002" "resolved" "review" "Resolved thread"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status=open"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:0001000100010001"* ]]
  [[ "$output" != *"l:0002000200020002"* ]]
}

@test "find-review: filter by goal" {
  write_thread "l:0001000100010001" "open" "review" "Review thread"
  write_thread "l:0002000200020002" "open" "discuss" "Discuss thread"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --goal=review"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:0001000100010001"* ]]
  [[ "$output" != *"l:0002000200020002"* ]]
}

@test "find-review: filter by group" {
  write_thread "l:0001000100010001" "open" "review" "Auth thread" "auth"
  write_thread "l:0002000200020002" "open" "review" "API thread" "api"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --group=auth"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:0001000100010001"* ]]
  [[ "$output" != *"l:0002000200020002"* ]]
}

@test "find-review: deps-of returns dependency threads" {
  write_thread "l:dep001" "open" "review" "Fix first"
  write_thread "l:main001" "open" "review" "Fix after dep" "" "l:dep001"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of=l:main001"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:dep001"* ]]
}

@test "find-review: deps-of with multiple dependencies" {
  write_thread "l:dep001" "open" "review" "Dep A"
  write_thread "l:dep002" "resolved" "review" "Dep B"
  write_thread "l:main001" "open" "review" "Main" "" "l:dep001,l:dep002"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of=l:main001"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"l:dep001"* ]]
  [[ "$output" == *"l:dep002"* ]]
}

@test "find-review: deps-of with no dependencies returns empty" {
  write_thread "l:standalone" "open" "review" "No deps"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of=l:standalone"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: deps-of nonexistent thread returns error" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of=l:nonexistent"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "find-review: blocked-by returns threads that depend on target" {
  write_thread "l:blocker" "open" "review" "Fix first"
  write_thread "l:blocked" "open" "review" "Fix after" "" "l:blocker"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --blocked-by=l:blocker"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:blocked"* ]]
}

@test "find-review: blocked-by with multiple dependents" {
  write_thread "l:blocker" "open" "review" "Fix first"
  write_thread "l:blocked1" "open" "review" "Blocked 1" "" "l:blocker"
  write_thread "l:blocked2" "open" "review" "Blocked 2" "" "l:blocker"
  write_thread "l:notblocked" "open" "review" "Not blocked"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --blocked-by=l:blocker"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"l:blocked1"* ]]
  [[ "$output" == *"l:blocked2"* ]]
  [[ "$output" != *"l:notblocked"* ]]
}

@test "find-review: refs-of returns referenced threads" {
  write_thread "l:target" "resolved" "review" "Context"
  write_thread_with_refs "l:source" "open" "review" "Uses context" "l:target"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --refs-of=l:source"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:target"* ]]
}

@test "find-review: referenced-by returns threads that reference target" {
  write_thread "l:target" "resolved" "review" "Context"
  write_thread_with_refs "l:source1" "open" "review" "Uses context" "l:target"
  write_thread "l:source2" "open" "review" "No refs"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --referenced-by=l:target"
  [ "$status" -eq 0 ]
  [[ "$output" == *"l:source1"* ]]
  [[ "$output" != *"l:source2"* ]]
}

@test "find-review: empty .nota directory" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status=open"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: no .nota directory exits cleanly" {
  rmdir "$tmpdir/.nota"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status=open"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

# ============================================================
# thread-add.sh
# ============================================================

@test "thread-add: adds comment to thread" {
  write_thread "l:0001000100010001" "open" "review" "Original message"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' l:0001000100010001 'New comment'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Added comment"* ]]

  # Verify comment was added
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:0001000100010001"
  [[ "$output" == *"New comment"* ]]
}

@test "thread-add: thread not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' l:0000000000000000 'Comment'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "thread-add: empty message fails" {
  write_thread "l:0001000100010001" "open" "review" "Original"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' l:0001000100010001 ''"
  [ "$status" -eq 1 ]
  [[ "$output" == *"empty"* ]]
}

@test "thread-add: reads from stdin with --file=-" {
  write_thread "l:0001000100010001" "open" "review" "Original"
  run bash -c "cd '$tmpdir' && echo 'Stdin message' | bash '$SCRIPT_DIR/thread-add.sh' l:0001000100010001 --file=-"
  [ "$status" -eq 0 ]

  # Verify the stdin content was recorded
  run bash -c "cd '$tmpdir' && cat .nota/*.xml"
  [[ "$output" == *"Stdin message"* ]]
}

@test "thread-add: records author as agent by default" {
  write_thread "l:0001000100010001" "open" "review" "Original"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' l:0001000100010001 'Agent reply'"
  [ "$status" -eq 0 ]

  run bash -c "cd '$tmpdir' && cat .nota/*.xml"
  [[ "$output" == *'author="agent"'* ]]
}

@test "thread-add: --human records author as the git user" {
  write_thread "l:0001000100010001" "open" "review" "Original"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-add.sh' --human l:0001000100010001 'Human reply'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Added comment"* ]]

  # The new comment must not be attributed to the agent.
  run bash -c "cd '$tmpdir' && cat .nota/*.xml"
  [[ "$output" != *'author="agent"'* ]]
}

@test "thread-add: --human still passes through other flags" {
  write_thread "l:0001000100010001" "open" "review" "Original"
  run bash -c "cd '$tmpdir' && echo 'Stdin human' | bash '$SCRIPT_DIR/thread-add.sh' --human l:0001000100010001 --file=-"
  [ "$status" -eq 0 ]

  run bash -c "cd '$tmpdir' && cat .nota/*.xml"
  [[ "$output" == *"Stdin human"* ]]
  # The comment must not be attributed to the agent when --human is used.
  [[ "$output" != *'author="agent"'* ]]
}

# ============================================================
# thread-resolve.sh
# ============================================================

@test "thread-resolve: marks thread resolved" {
  write_thread "l:0001000100010001" "open" "review" "Fix this"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-resolve.sh' l:0001000100010001"
  [ "$status" -eq 0 ]
  [[ "$output" == *"resolved"* ]]

  # Verify status changed
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/read-open.sh' l:0001000100010001"
  [[ "$output" == *"Status: resolved"* ]]
}

@test "thread-resolve: thread not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-resolve.sh' l:0000000000000000"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

# ============================================================
# thread-spawn.sh
# ============================================================

@test "thread-spawn: creates child thread" {
  write_thread "l:aabbccddaabbccdd" "open" "review" "Parent thread" "mygroup"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-spawn.sh' l:aabbccddaabbccdd 'Child concern'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Created child thread"* ]]
  [[ "$output" == *"parent: l:aabbccddaabbccdd"* ]]
}

@test "thread-spawn: parent not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-spawn.sh' l:0000000000000000 'Child'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "thread-spawn: records author as agent by default" {
  write_thread "l:aabbccddaabbccdd" "open" "review" "Parent thread" "mygroup"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-spawn.sh' l:aabbccddaabbccdd 'Child concern'"
  [ "$status" -eq 0 ]

  run bash -c "cd '$tmpdir' && grep -l 'Child concern' .nota/*.xml | xargs cat"
  [[ "$output" == *'author="agent"'* ]]
}

@test "thread-spawn: child inherits group from parent" {
  write_thread "l:aabbccddaabbccdd" "open" "review" "Parent" "pr-123"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/thread-spawn.sh' l:aabbccddaabbccdd 'Child'"
  [ "$status" -eq 0 ]

  # The child should have inherited the group - check via listing
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --group=pr-123"
  [ "${#lines[@]}" -eq 2 ]  # parent + child
}
