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

# Helper: create a tracking file
write_tracking() {
  local name="$1"
  shift
  cat > "$tmpdir/.nota/$name" <<< "$@"
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

@test "list-open: resolved file not listed" {
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
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
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  cat > "$tmpdir/.nota/api.md" <<'EOF'
---
status: resolved
group: api
---

## [resolved] review — src/api/handler.go:10

Done
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/list-open.sh'"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/auth.md" ]
}

@test "list-open: multiple open files" {
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
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix token expiry
EOF
  run run_validate "$tmpdir/.nota/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: valid resolved file" {
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: resolved
group: auth
---

## [resolved] review — src/auth/login.go:42

> Fixed
EOF
  run run_validate "$tmpdir/.nota/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: valid file with mixed sections" {
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
---

## [resolved] review — src/auth/login.go:42

> Fixed

## discuss — src/auth/middleware.go:18

Still open
EOF
  run run_validate "$tmpdir/.nota/auth.md"
  [ "$status" -eq 0 ]
}

@test "validate: invalid status value" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
---
status: pending
---

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Invalid status"* ]]
}

@test "validate: missing status field" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
---
group: auth
---

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Missing"* ]]
}

@test "validate: invalid section heading" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
---
status: open
---

## fix this thing

Not a valid heading
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Invalid section headings"* ]]
}

@test "validate: status open but all sections resolved" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
---
status: open
---

## [resolved] review — src/foo.go:1

Fixed
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"status is still"* ]]
}

@test "validate: status resolved but has open sections" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

Fixed

## review — src/foo.go:10

Still open
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"open sections"* ]]
}

@test "validate: non-.nota file is skipped" {
  cat > "$tmpdir/random.md" <<'EOF'
no frontmatter at all
EOF
  run run_validate "$tmpdir/random.md"
  [ "$status" -eq 0 ]
}

@test "validate: missing frontmatter" {
  cat > "$tmpdir/.nota/bad.md" <<'EOF'
no frontmatter here

## review — src/foo.go:1

Something
EOF
  run run_validate "$tmpdir/.nota/bad.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"Missing frontmatter"* ]]
}

@test "validate: accepts extension tags in headings" {
  cat > "$tmpdir/.nota/ext.md" <<'EOF'
---
status: open
---

## impl — src/foo.go:1

Implement this

## refactor — src/foo.go:10

Refactor that

## critique — src/foo.go:20

Challenge this

## propose — src/foo.go:30

Suggest alternative

## test — src/foo.go:40

Write tests

## doc — src/foo.go:50

Document this
EOF
  run run_validate "$tmpdir/.nota/ext.md"
  [ "$status" -eq 0 ]
}

@test "validate: accepts depends-on and references in frontmatter" {
  cat > "$tmpdir/.nota/dep-target.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/ref-target.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/bar.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/main.md" <<'EOF'
---
status: open
depends-on:
  - dep-target
references:
  - ref-target
tags:
  - security
  - auth
---

## review — src/baz.go:1

Fix this
EOF
  run run_validate "$tmpdir/.nota/main.md"
  [ "$status" -eq 0 ]
}

@test "validate: warns on missing depends-on target" {
  cat > "$tmpdir/.nota/bad-dep.md" <<'EOF'
---
status: open
depends-on:
  - nonexistent
---

## review — src/foo.go:1

Fix this
EOF
  run run_validate "$tmpdir/.nota/bad-dep.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"depends-on target 'nonexistent' not found"* ]]
}

@test "validate: warns on missing references target" {
  cat > "$tmpdir/.nota/bad-ref.md" <<'EOF'
---
status: open
references:
  - nonexistent
---

## review — src/foo.go:1

Fix this
EOF
  run run_validate "$tmpdir/.nota/bad-ref.md"
  [ "$status" -eq 1 ]
  [[ "$output" == *"references target 'nonexistent' not found"* ]]
}

# ============================================================
# find-review.sh
# ============================================================

@test "find-review: lookup by name" {
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: open
group: auth
---

## review — src/auth/login.go:42

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' auth"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/auth.md" ]
}

@test "find-review: name not found" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' nonexistent"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: filter by status" {
  cat > "$tmpdir/.nota/open1.md" <<'EOF'
---
status: open
---

## review — src/foo.go:1

Fix
EOF
  cat > "$tmpdir/.nota/done1.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/bar.go:1

> Fixed
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/open1.md" ]
}

@test "find-review: filter by tag" {
  cat > "$tmpdir/.nota/sec.md" <<'EOF'
---
status: open
tags:
  - security
---

## review — src/foo.go:1

Fix
EOF
  cat > "$tmpdir/.nota/other.md" <<'EOF'
---
status: open
tags:
  - perf
---

## review — src/bar.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --tag security"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/sec.md" ]
}

@test "find-review: refs-of returns referenced files" {
  cat > "$tmpdir/.nota/target.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/source.md" <<'EOF'
---
status: open
references:
  - target
---

## review — src/bar.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --refs-of source"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/target.md" ]
}

@test "find-review: deps-of returns dependency files" {
  cat > "$tmpdir/.nota/dep.md" <<'EOF'
---
status: open
---

## review — src/foo.go:1

Fix first
EOF
  cat > "$tmpdir/.nota/main.md" <<'EOF'
---
status: open
depends-on:
  - dep
---

## review — src/bar.go:1

Fix after dep
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of main"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/dep.md" ]
}

@test "find-review: referenced-by returns files that reference a name" {
  cat > "$tmpdir/.nota/target.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/a.md" <<'EOF'
---
status: open
references:
  - target
---

## review — src/bar.go:1

Fix
EOF
  cat > "$tmpdir/.nota/b.md" <<'EOF'
---
status: open
---

## review — src/baz.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --referenced-by target"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/a.md" ]
}

@test "find-review: blocked-by returns files that depend on a name" {
  cat > "$tmpdir/.nota/blocker.md" <<'EOF'
---
status: open
---

## review — src/foo.go:1

Fix first
EOF
  cat > "$tmpdir/.nota/blocked.md" <<'EOF'
---
status: open
depends-on:
  - blocker
---

## review — src/bar.go:1

Fix after
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --blocked-by blocker"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/blocked.md" ]
}

@test "find-review: deps-of with multiple dependencies" {
  cat > "$tmpdir/.nota/dep-a.md" <<'EOF'
---
status: open
---

## review — src/a.go:1

Fix A
EOF
  cat > "$tmpdir/.nota/dep-b.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/b.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/main.md" <<'EOF'
---
status: open
depends-on:
  - dep-a
  - dep-b
---

## review — src/main.go:1

Fix after both
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of main"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"dep-a.md"* ]]
  [[ "$output" == *"dep-b.md"* ]]
}

@test "find-review: deps-of skips missing target silently" {
  cat > "$tmpdir/.nota/main.md" <<'EOF'
---
status: open
depends-on:
  - exists
  - gone
---

## review — src/main.go:1

Fix
EOF
  cat > "$tmpdir/.nota/exists.md" <<'EOF'
---
status: open
---

## review — src/exists.go:1

Here
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of main"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 1 ]
  [ "$output" = "$tmpdir/.nota/exists.md" ]
}

@test "find-review: deps-of with no dependencies returns empty" {
  cat > "$tmpdir/.nota/standalone.md" <<'EOF'
---
status: open
---

## review — src/foo.go:1

No deps
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of standalone"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: deps-of nonexistent file returns empty" {
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --deps-of nonexistent"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: refs-of with multiple references" {
  cat > "$tmpdir/.nota/ref-a.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/a.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/ref-b.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/b.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/source.md" <<'EOF'
---
status: open
references:
  - ref-a
  - ref-b
---

## review — src/main.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --refs-of source"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"ref-a.md"* ]]
  [[ "$output" == *"ref-b.md"* ]]
}

@test "find-review: refs-of skips missing target silently" {
  cat > "$tmpdir/.nota/exists.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/foo.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/source.md" <<'EOF'
---
status: open
references:
  - exists
  - gone
---

## review — src/main.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --refs-of source"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 1 ]
  [ "$output" = "$tmpdir/.nota/exists.md" ]
}

@test "find-review: blocked-by with multiple dependents" {
  cat > "$tmpdir/.nota/blocker.md" <<'EOF'
---
status: open
---

## review — src/blocker.go:1

Fix first
EOF
  cat > "$tmpdir/.nota/a.md" <<'EOF'
---
status: open
depends-on:
  - blocker
---

## review — src/a.go:1

Blocked
EOF
  cat > "$tmpdir/.nota/b.md" <<'EOF'
---
status: open
depends-on:
  - blocker
---

## review — src/b.go:1

Also blocked
EOF
  cat > "$tmpdir/.nota/c.md" <<'EOF'
---
status: open
---

## review — src/c.go:1

Not blocked
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --blocked-by blocker"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"a.md"* ]]
  [[ "$output" == *"b.md"* ]]
}

@test "find-review: referenced-by with multiple referrers" {
  cat > "$tmpdir/.nota/ctx.md" <<'EOF'
---
status: resolved
---

## [resolved] review — src/ctx.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/a.md" <<'EOF'
---
status: open
references:
  - ctx
---

## review — src/a.go:1

Fix
EOF
  cat > "$tmpdir/.nota/b.md" <<'EOF'
---
status: open
references:
  - ctx
---

## review — src/b.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --referenced-by ctx"
  [ "$status" -eq 0 ]
  [ "${#lines[@]}" -eq 2 ]
  [[ "$output" == *"a.md"* ]]
  [[ "$output" == *"b.md"* ]]
}

@test "find-review: filter by multiple tags matches file with matching tag" {
  cat > "$tmpdir/.nota/multi.md" <<'EOF'
---
status: open
tags:
  - security
  - auth
---

## review — src/foo.go:1

Fix
EOF
  cat > "$tmpdir/.nota/other.md" <<'EOF'
---
status: open
tags:
  - perf
---

## review — src/bar.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --tag auth"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/multi.md" ]
}

@test "find-review: no tag in frontmatter skipped by tag filter" {
  cat > "$tmpdir/.nota/notags.md" <<'EOF'
---
status: open
---

## review — src/foo.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --tag security"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}

@test "find-review: combined status and tag filter" {
  cat > "$tmpdir/.nota/match.md" <<'EOF'
---
status: open
tags:
  - security
---

## review — src/foo.go:1

Fix
EOF
  cat > "$tmpdir/.nota/wrong-status.md" <<'EOF'
---
status: resolved
tags:
  - security
---

## [resolved] review — src/bar.go:1

> Fixed
EOF
  cat > "$tmpdir/.nota/wrong-tag.md" <<'EOF'
---
status: open
tags:
  - perf
---

## review — src/baz.go:1

Fix
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open --tag security"
  [ "$status" -eq 0 ]
  [ "$output" = "$tmpdir/.nota/match.md" ]
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

@test "find-review: combined name and status filter" {
  cat > "$tmpdir/.nota/auth.md" <<'EOF'
---
status: resolved
group: auth
---

## [resolved] review — src/auth/login.go:42

> Fixed
EOF
  run bash -c "cd '$tmpdir' && bash '$SCRIPT_DIR/find-review.sh' --status open auth"
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
}
