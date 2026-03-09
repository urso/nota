---
description: Remove review comments from code without extracting
allowed-tools: Bash
argument-hint: [optional flags e.g. "--staged" or file paths]
---

# aireview-delete — Delete Comments from Code

Remove all review comments from source code without extracting them to tracking files. The comments are deleted permanently.

**Confirm with the user before running.** Ask: "This will delete all review comments from the source code without saving them. Proceed?"

If the user confirms:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/aireview.sh delete --all $ARGUMENTS 2>&1
```

Report how many files were modified.
