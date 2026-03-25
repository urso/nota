#!/bin/bash
# PostToolUse hook: validates .nota/ tracking files after Edit.
# Receives hook input as JSON on stdin.
# Exits 0 if valid or not applicable, non-zero with error message if invalid.

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

if [ -z "$file" ] || [ ! -f "$file" ]; then
  exit 0
fi

# Only validate files in .nota/
case "$file" in
  .nota/*|*/.nota/*) ;;
  *) exit 0 ;;
esac

errors=""

# Check frontmatter exists
if ! head -1 "$file" | grep -q '^---$'; then
  errors="${errors}Missing frontmatter (file must start with ---)\n"
fi

# Extract and validate frontmatter
frontmatter=$(awk '/^---$/{c++;next} c==1{print} c>=2{exit}' "$file")

if [ -z "$frontmatter" ]; then
  errors="${errors}Empty or missing frontmatter\n"
else
  # Check status field exists and is valid
  status=$(echo "$frontmatter" | grep -E '^status:' | head -1 | sed 's/^status:[[:space:]]*//' | tr -d '[:space:]')
  if [ -z "$status" ]; then
    errors="${errors}Missing 'status' field in frontmatter\n"
  elif [ "$status" != "open" ] && [ "$status" != "resolved" ]; then
    errors="${errors}Invalid status '${status}' — must be 'open' or 'resolved'\n"
  fi

  # Resolve .nota/ directory from the file path itself
  nota_dir=$(cd "$(dirname "$file")" && pwd -P)

  # Validate depends-on and references targets exist
  if [ -n "$nota_dir" ]; then
    for field in depends-on references; do
      targets=$(echo "$frontmatter" | awk -v key="$field" '
        $0 ~ "^"key":" { found=1; next }
        found && /^[[:space:]]*-[[:space:]]/ { sub(/^[[:space:]]*-[[:space:]]*/, ""); print; next }
        found && /^[a-z]/ { exit }
      ')
      for target in $targets; do
        if [ ! -f "${nota_dir}/${target}.md" ]; then
          errors="${errors}${field} target '${target}' not found (${nota_dir}/${target}.md)\n"
        fi
      done
    done
  fi
fi

# Validate section headings — allow any lowercase tag name (built-in + extensions)
bad_headings=$(grep '^## ' "$file" | grep -v -E '^## (\[(resolved|wontfix)\] )?[a-z][a-z0-9_-]* — ')
if [ -n "$bad_headings" ]; then
  errors="${errors}Invalid section headings:\n${bad_headings}\nExpected format: ## [resolved|wontfix]? <tag> — file:line\n"
fi

# Check status consistency
total_sections=$(grep -c '^## ' "$file" 2>/dev/null) || total_sections=0
resolved_sections=$(grep -c '^## \[\(resolved\|wontfix\)\]' "$file" 2>/dev/null) || resolved_sections=0

if [ "$total_sections" -gt 0 ] && [ "$total_sections" -eq "$resolved_sections" ] && [ "$status" = "open" ]; then
  errors="${errors}All sections are resolved/wontfix but status is still 'open' — update frontmatter to 'status: resolved'\n"
fi

if [ "$total_sections" -gt 0 ] && [ "$total_sections" -ne "$resolved_sections" ] && [ "$status" = "resolved" ]; then
  errors="${errors}Status is 'resolved' but there are open sections — status should be 'open'\n"
fi

if [ -n "$errors" ]; then
  echo "Tracking file validation failed for ${file}:"
  echo -e "$errors"
  exit 1
fi
