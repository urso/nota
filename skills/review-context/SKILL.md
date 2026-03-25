---
description: "This skill should be used when the user asks to 'show open reviews', 'continue the review', 'resume review work', 'what reviews are pending', or after context compaction or session handover. Loads existing review state from .nota/ tracking files — does not extract new comments from source code."
version: 1.0.0
user-invocable: false
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

## Step 3: Check relationships

For each open file, check if it has `depends-on` or `references` in its frontmatter. If so, read those files too for context:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --refs-of <name>
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --deps-of <name>
```

Where `<name>` is the filename stem (e.g. `auth` for `.nota/auth.md`).

## Tracking file format

Each `.nota/*.md` file:

```markdown
---
status: open
group: optional-group-name
depends-on:
  - other-review-file
references:
  - resolved-review-file
tags:
  - security
---

## tag — file.go:LINE

Review comment body...
```

- **status**: `open` or `resolved`
- **group**: optional name from extraction (e.g. `review(auth):` → group `auth`)
- **depends-on**: list of filename stems that should be addressed before this one
- **references**: list of filename stems with relevant context (may be resolved)
- **tags**: list of labels for grouping/filtering
- **tag** in headings identifies intent (e.g. `review`, `discuss`, `explain`, `impl`, `critique`, or custom tags). Run `bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh behavior` for all known tags and behaviors.
- Resolved sections have `[resolved]` or `[wontfix]` prepended to the heading
- A resolution note appears below resolved headings as a blockquote

## After loading

Summarize open items including dependency relationships, continue addressing in-progress work, or answer questions about review status. Never mark an item resolved without user confirmation.
