---
description: Overview of open and resolved reviews
allowed-tools: Bash, Read
---

# nota-status — Review Status Overview

```
!`ls "$(git rev-parse --show-toplevel)/.nota"/*.md 2>/dev/null || echo "NO_TRACKING_FILES"`
```

If no tracking files exist, report no reviews and stop.

Read each file's frontmatter and section headings. Present a summary table:

| File | Group | Status | Open | Resolved | Wontfix |

Then: total counts, classification by theme, and suggestion of what to tackle next.
