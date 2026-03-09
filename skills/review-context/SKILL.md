---
name: Review Context
description: "Use this skill when you need to load or resume work on code reviews from .aireview/ tracking files — for example after context compaction, session handover, or when the user references open reviews. Trigger when: user mentions reviews, asks to continue review work, or you need to understand the current review state. Do NOT use this skill to extract new comments from source code."
version: 1.0.0
---

# Review Context — Loading Open Reviews

You need to load the current state of code reviews from `.aireview/` tracking files. This is useful after context compaction, handover, or any time you've lost context about ongoing reviews.

## Step 1: Find the review directory

The `.aireview/` directory lives at the git repository root:

```bash
dir="$(git rev-parse --show-toplevel)/.aireview"
```

If the directory doesn't exist, there are no reviews to load.

## Step 2: List open tracking files

Run the list-open script to find files with `status: open`:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh
```

If no files are listed, there are no open reviews.

## Step 3: Read open sections from each file

For each file from Step 2, read only the unresolved sections:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

This filters out `[resolved]` and `[wontfix]` sections, showing only items that still need attention.

## Tracking file format

Each `.aireview/*.md` file has this structure:

```markdown
---
source: path/to/source/file.go
group: optional-group-name
status: open
---

## tag: Short title (file.go:LINE)

Review comment body...
```

- **source**: the file where the comment originated
- **group**: optional grouping for related reviews
- **status**: `open` or `resolved`
- **Section headings** (`##`): one per review item
  - **tag** is one of: `review` (fix this), `discuss` (debate this), `explain` (clarify this)
  - Resolved sections get `[resolved]` or `[wontfix]` prepended to the heading
  - A resolution note appears below resolved headings as a blockquote

## How to use the loaded context

After loading reviews, you understand the current review state. You can then:

- **Summarize** open items for the user
- **Continue addressing** items that were in progress before compaction/handover
- **Answer questions** about review status
- **Mark items resolved** by editing the tracking file: prepend `[resolved]` or `[wontfix]` to the section heading and add a `> Fixed: ...` note

**Important**: Never mark an item resolved without user confirmation.
