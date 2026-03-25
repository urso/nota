#!/bin/bash
# Query .nota/ tracking files by name, status, tag, references, or dependencies.
# Outputs absolute paths of matching files.
#
# Usage:
#   find-review.sh <name>                 Find file by filename stem (e.g. "auth" → .nota/auth.md)
#   find-review.sh --status open          List files with given status
#   find-review.sh --tag security         List files containing a given tag
#   find-review.sh --refs-of <name>       List files that <name> references
#   find-review.sh --deps-of <name>       List files that <name> depends on
#   find-review.sh --referenced-by <name> List files that reference <name>
#   find-review.sh --blocked-by <name>    List files that depend on <name>
#
# Options can be combined. All filters are ANDed.

set -euo pipefail

# Resolve .nota/ directory
root=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [ -z "$root" ]; then
  d=$(pwd)
  while [ "$d" != "/" ]; do
    if [ -d "$d/.nota" ]; then
      root="$d"
      break
    fi
    d=$(dirname "$d")
  done
  root="${root:-$(pwd)}"
fi

dir="$root/.nota"

if [ ! -d "$dir" ]; then
  exit 0
fi

# Parse arguments
name=""
filter_status=""
filter_tag=""
refs_of=""
deps_of=""
referenced_by=""
blocked_by=""

while [ $# -gt 0 ]; do
  case "$1" in
    --status)
      filter_status="$2"; shift 2 ;;
    --tag)
      filter_tag="$2"; shift 2 ;;
    --refs-of)
      refs_of="$2"; shift 2 ;;
    --deps-of)
      deps_of="$2"; shift 2 ;;
    --referenced-by)
      referenced_by="$2"; shift 2 ;;
    --blocked-by)
      blocked_by="$2"; shift 2 ;;
    -*)
      echo "Unknown option: $1" >&2; exit 1 ;;
    *)
      name="$1"; shift ;;
  esac
done

# Helper: extract frontmatter from a file (between first and second ---)
get_frontmatter() {
  awk '/^---$/{c++;next} c==1{print} c>=2{exit}' "$1"
}

# Helper: extract a scalar field from frontmatter
get_field() {
  echo "$1" | grep -E "^${2}:" | head -1 | sed "s/^${2}:[[:space:]]*//"
}

# Helper: extract list items from frontmatter (YAML list under a key)
# Handles both "key: [a, b]" inline and multi-line "- item" forms
get_list() {
  local fm="$1" key="$2"
  # Try inline form first: key: [a, b, c]
  local inline
  inline=$(echo "$fm" | grep -E "^${key}:[[:space:]]*\[" | head -1 | sed "s/^${key}:[[:space:]]*\[//;s/\][[:space:]]*$//")
  if [ -n "$inline" ]; then
    echo "$inline" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
    return
  fi
  # Multi-line form: key:\n  - item1\n  - item2
  awk -v key="$key" '
    $0 ~ "^"key":" { found=1; next }
    found && /^[[:space:]]*-[[:space:]]/ { sub(/^[[:space:]]*-[[:space:]]*/, ""); print; next }
    found && /^[a-z]/ { exit }
  ' <<< "$fm"
}

# If looking up refs-of or deps-of, first resolve the source file
if [ -n "$refs_of" ]; then
  source_file="$dir/${refs_of}.md"
  if [ ! -f "$source_file" ]; then
    exit 0
  fi
  fm=$(get_frontmatter "$source_file")
  targets=$(get_list "$fm" "references")
  if [ -z "$targets" ]; then
    exit 0
  fi
  for t in $targets; do
    f="$dir/${t}.md"
    [ -f "$f" ] && echo "$f"
  done
  exit 0
fi

if [ -n "$deps_of" ]; then
  source_file="$dir/${deps_of}.md"
  if [ ! -f "$source_file" ]; then
    exit 0
  fi
  fm=$(get_frontmatter "$source_file")
  targets=$(get_list "$fm" "depends-on")
  if [ -z "$targets" ]; then
    exit 0
  fi
  for t in $targets; do
    f="$dir/${t}.md"
    [ -f "$f" ] && echo "$f"
  done
  exit 0
fi

# If looking up reverse relations (who references/depends-on <name>)
if [ -n "$referenced_by" ]; then
  for f in "$dir"/*.md; do
    [ -f "$f" ] || continue
    fm=$(get_frontmatter "$f")
    refs=$(get_list "$fm" "references")
    for r in $refs; do
      if [ "$r" = "$referenced_by" ]; then
        echo "$f"
        break
      fi
    done
  done
  exit 0
fi

if [ -n "$blocked_by" ]; then
  for f in "$dir"/*.md; do
    [ -f "$f" ] || continue
    fm=$(get_frontmatter "$f")
    deps=$(get_list "$fm" "depends-on")
    for d in $deps; do
      if [ "$d" = "$blocked_by" ]; then
        echo "$f"
        break
      fi
    done
  done
  exit 0
fi

# Direct name lookup
if [ -n "$name" ]; then
  f="$dir/${name}.md"
  if [ -f "$f" ]; then
    # Apply additional filters if given
    if [ -n "$filter_status" ] || [ -n "$filter_tag" ]; then
      fm=$(get_frontmatter "$f")
      if [ -n "$filter_status" ]; then
        status=$(get_field "$fm" "status" | tr -d '[:space:]')
        [ "$status" != "$filter_status" ] && exit 0
      fi
      if [ -n "$filter_tag" ]; then
        tags=$(get_list "$fm" "tags")
        found=0
        for t in $tags; do
          [ "$t" = "$filter_tag" ] && found=1 && break
        done
        [ "$found" -eq 0 ] && exit 0
      fi
    fi
    echo "$f"
  fi
  exit 0
fi

# Filter all files
for f in "$dir"/*.md; do
  [ -f "$f" ] || continue

  fm=$(get_frontmatter "$f")

  if [ -n "$filter_status" ]; then
    status=$(get_field "$fm" "status" | tr -d '[:space:]')
    [ "$status" != "$filter_status" ] && continue
  fi

  if [ -n "$filter_tag" ]; then
    tags=$(get_list "$fm" "tags")
    found=0
    for t in $tags; do
      [ "$t" = "$filter_tag" ] && found=1 && break
    done
    [ "$found" -eq 0 ] && continue
  fi

  echo "$f"
done
