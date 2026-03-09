---
description: Peek at review comments in code without extracting
allowed-tools: Bash
argument-hint: [optional flags e.g. "--staged" or file paths]
---

# aireview-list — List Comments in Code

Show the developer what review comments exist in the source code without extracting or deleting them. This is a read-only peek.

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/aireview.sh list --all $ARGUMENTS 2>&1`
```

Present the output to the user as-is.
