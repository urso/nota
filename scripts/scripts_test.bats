#!/usr/bin/env bats

SCRIPT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)"

setup() {
  tmpdir=$(cd "$(mktemp -d)" && pwd -P)
  git init -q "$tmpdir"
  mkdir -p "$tmpdir/.aireview"
}

teardown() {
  rm -rf "$tmpdir"
}

# Helper: create a tracking file
write_tracking() {
  local name="$1"
  shift
  cat > "$tmpdir/.aireview/$name" <<< "$@"
}

# Helper: run validate-tracking with a JSON input
run_validate() {
  local file="$1"
  echo "{\"tool_input\":{\"file_path\":\"$file\"}}" | bash "$SCRIPT_DIR/validate-tracking.sh"
}

# ============================================================
# list-open.sh
# ============================================================

@test "list-open: no .aireview directory" {
  rmdir "$tmpdir/.aireview"
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: empty .aireview directory" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: one open file" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.aireview/auth.md" ]
}

@test "list-open: resolved file not listed" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: resolved
group: auth
---

## [resolved] review — src/auth/login.go:42

Fixed
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "list-open: mixed open and resolved" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  cat > "$tmpdir/.aireview/api.md" <<'EOF'
---
status: resolved
group: api
---

## [resolved] review — src/api/handler.go:10

Done
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.aireview/auth.md" ]
}

@test "list-open: multiple open files" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: open
---

## review — src/auth/login.go:42

Fix
EOF
  cat > "$tmpdir/.aireview/api.md" <<'EOF'
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
# read-open.sh
# ============================================================

@test "read-open: all sections open" {
  cat > "$tmpdir/test.md" <<'EOF'
---
status: open
---

## review — src/auth/login.go:42

Fix token expiry

## discuss — src/auth/middleware.go:18

Should we validate on every request?
EOF
  run bash "$SCRIPT_DIR/read-open.sh" "$tmpdir/test.md"
  [ "$status" -eq 0 ]
  [[ "$output" == *"## review — src/auth/login.go:42"* ]]
  [[ "$output" == *"## discuss — src/auth/middleware.go:18"* ]]
}

@test "read-open: strips resolved sections" {
  cat > "$tmpdir/test.md" <<'EOF'
---
status: open
---

## [resolved] review — src/auth/login.go:42

> Fixed: changed <= to <

## discuss — src/auth/middleware.go:18

Should we validate on every request?
EOF
  run bash "$SCRIPT_DIR/read-open.sh" "$tmpdir/test.md"
  [ "$status" -eq 0 ]
  [[ "$output" != *"[resolved]"* ]]
  [[ "$output" == *"## discuss — src/auth/middleware.go:18"* ]]
}

@test "read-open: strips wontfix sections" {
  cat > "$tmpdir/test.md" <<'EOF'
---
status: open
---

## [wontfix] discuss — src/auth/middleware.go:18

> Not worth changing

## review — src/auth/login.go:42

Fix this
EOF
  run bash "$SCRIPT_DIR/read-open.sh" "$tmpdir/test.md"
  [ "$status" -eq 0 ]
  [[ "$output" != *"[wontfix]"* ]]
  [[ "$output" == *"## review — src/auth/login.go:42"* ]]
}

@test "read-open: all sections resolved gives only frontmatter" {
  cat > "$tmpdir/test.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/auth/login.go:42

> Fixed

## [wontfix] discuss — src/auth/middleware.go:18

> Not worth changing
EOF
  run bash "$SCRIPT_DIR/read-open.sh" "$tmpdir/test.md"
  [ "$status" -eq 0 ]
  [[ "$output" != *"## "* ]]
}

@test "read-open: missing file exits non-zero" {
  run bash "$SCRIPT_DIR/read-open.sh" "/nonexistent/file.md"
  [ "$status" -eq 1 ]
}

@test "read-open: no arguments exits non-zero" {
  run bash "$SCRIPT_DIR/read-open.sh"
  [ "$status" -eq 1 ]
}

# ============================================================
# validate-tracking.sh
# ============================================================

@test "validate: valid open file" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  run run_validate "$tmpdir/.aireview/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: valid resolved file" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: resolved
group: auth
---

## [resolved] review — src/auth/login.go:42

> Fixed
EOF
  run run_validate "$tmpdir/.aireview/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: valid file with mixed sections" {
  cat > "$tmpdir/.aireview/auth.md" <<'EOF'
---
status: open
---

## [resolved] review — src/auth/login.go:42

> Fixed

## discuss — src/auth/middleware.go:18

Still open
EOF
  run run_validate "$tmpdir/.aireview/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: invalid status value" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
---
status: pending
---

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Invalid status"* ]]
}

@test "validate: missing status field" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
---
group: auth
---

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Missing"* ]]
}

@test "validate: invalid section heading" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
---
status: open
---

## fix this thing

Not a valid heading
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Invalid section headings"* ]]
}

@test "validate: status open but all sections resolved" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
---
status: open
---

## [resolved] review — src/foo.go:1

Fixed
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"status is still"* ]]
}

@test "validate: status resolved but has open sections" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

Fixed

## review — src/foo.go:10

Still open
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"open sections"* ]]
}

@test "validate: non-.aireview file is skipped" {
  cat > "$tmpdir/random.md" <<'EOF'
no frontmatter at all
EOF
  run run_validate "$tmpdir/random.md"
  [ "$status" -eq 0 ]
}

@test "validate: missing frontmatter" {
  cat > "$tmpdir/.aireview/bad.md" <<'EOF'
no frontmatter here

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.aireview/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Missing frontmatter"* ]]
}
