---
description: Remove resolved review files from .nota/
allowed-tools: Bash
---

# nota-cleanup — Clean Up Resolved Reviews

Remove tracking files from `.nota/` that have `status: resolved` in their frontmatter.

## Step 1: Find resolved files

```bash
dir="$(git rev-parse --show-toplevel)/.nota"
for f in "$dir"/*.md; do
  [ -f "$f" ] || continue
  if awk '/^---$/{c++;next} c==1&&/^status:[[:space:]]*resolved/{found=1;exit} c>=2{exit} END{exit !found}' "$f"; then
    echo "$f"
  fi
done
```

If no resolved files found, tell the user there are no resolved reviews to clean up and stop.

## Step 2: Confirm and delete

List the resolved files and ask the user: "Remove these resolved review files?"

If confirmed, delete them and report what was removed.
