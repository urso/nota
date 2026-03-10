---
name: Review Context
description: "This skill should be used when the user asks to 'show open reviews', 'continue the review', 'resume review work', 'what reviews are pending', or after context compaction or session handover. Loads existing review state from .nota/ tracking files — does not extract new comments from source code."
version: 1.0.0
---

# Review Context — Loading Open Reviews

Load the current state of code reviews from `.nota/` tracking files. Useful after context compaction, handover, or when context about ongoing reviews has been lost.

## Step 1: List open tracking files

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh
```

If no files listed, there are no open reviews.

## Step 2: Read open sections

For each file from Step 1:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

This filters out `[resolved]` and `[wontfix]` sections.

## Tracking file format

Each `.nota/*.md` file:

```markdown
---
source: path/to/file.go
group: optional-group-name
status: open
---

## tag — file.go:LINE

Review comment body...
```

- **tag** identifies intent (e.g. `review`, `discuss`, `explain`, `impl`, `critique`, or custom tags). Run `bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh behavior` for all known tags and behaviors.
- Resolved sections have `[resolved]` or `[wontfix]` prepended to the heading
- A resolution note appears below resolved headings as a blockquote

## After loading

Summarize open items, continue addressing in-progress work, or answer questions about review status. Never mark an item resolved without user confirmation.
