---
description: Overview of open and resolved reviews
allowed-tools: Bash, Read
---

# aireview-status — Review Status Overview

Show the user an overview of all review tracking files.

## Step 1: List all tracking files

```
!`ls "$(git rev-parse --show-toplevel)/.aireview"/*.md 2>/dev/null || echo "NO_TRACKING_FILES"`
```

If no tracking files exist, tell the user there are no reviews (open or resolved) and stop.

## Step 2: Read each file

For each tracking file, read its frontmatter (`status`, `group`) and scan section headings to count:
- Open sections (headings without `[resolved]` or `[wontfix]`)
- Resolved sections (`[resolved]`)
- Wontfix sections (`[wontfix]`)

## Step 3: Present summary

Show a table with:

| File | Group | Status | Open | Resolved | Wontfix |
|------|-------|--------|------|----------|---------|

Then provide:
- Total counts across all files
- A brief classification by theme or area (group related files together)
- Suggest what to tackle next (e.g. "3 quick review fixes in auth.md, 1 open discussion in api.md")

Keep it concise. This is an overview, not a deep dive.
