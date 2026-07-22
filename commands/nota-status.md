---
description: Overview of open and resolved reviews
allowed-tools: Bash, Read
---

# nota-status — Review Status Overview

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh thread list 2>&1`
```

If no threads exist, report no reviews and stop.

The output is tab-separated: `ID STATUS GOAL TITLE`

Present a summary table:

| ID | Status | Goal | Group | Title |

Then: total counts by status and goal, dependency graph (which threads block others via `depends-on`), and suggestion of what to tackle next (prefer threads with no unsatisfied dependencies).

To check dependencies for a specific thread:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --deps-of <thread-id>
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --blocked-by <thread-id>
```
