---
description: "This skill should be used when the user asks to 'show open reviews', 'continue the review', 'resume review work', 'what reviews are pending', or after context compaction or session handover. Loads existing review state from .nota/ threads — does not extract new comments from source code."
version: 1.0.0
user-invocable: false
---

# Review Context — Loading Open Reviews

Load the current state of code reviews from `.nota/` thread files. Useful after context compaction, handover, or when context about ongoing reviews has been lost.

## Step 1: List open threads

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh
```

If no threads listed, there are no open reviews.

The output is tab-separated: `ID STATUS GOAL TITLE`

## Step 2: Read thread contents

For each thread from Step 1:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <thread-id>
```

This shows the full thread with all comments.

## Step 3: Check relationships

For each open thread, check if it has dependencies or references. If so, read those threads too for context:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --refs-of <thread-id>
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --deps-of <thread-id>
```

## Thread format

Each `.nota/*.xml` file contains a thread:

- **ID**: Unique identifier (e.g. `l:abc123...` for local, `gh:123` for GitHub)
- **Status**: `open`, `resolved`, or `wontfix`
- **Goal**: Intent — `review`, `discuss`, `impl`, `explain`, `refactor`, `test`, `doc`, `propose`, `critique`
- **Group**: Optional grouping name (e.g. `pr-487`, `auth-refactor`)
- **Tags**: Optional comma-separated labels for filtering
- **Anchor**: Optional code location (`file:line @ commit`)
- **depends-on**: Optional comma-separated thread IDs this thread is blocked by
- **Comments**: Ordered list of messages with author, timestamp, and content

Run `bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh behavior` for all known goals and their expected behaviors.

## After loading

Summarize open items including dependency relationships, continue addressing in-progress work, or answer questions about review status. Never mark a thread resolved without user confirmation.
