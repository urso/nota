---
description: Critically evaluate open reviews — check if reviewer is right
allowed-tools: Bash, Read
argument-hint: [optional thread ID or group to focus on]
---

# nota-evaluate — Critical Review Evaluation

Critically assess whether each review comment is valid. Flag false positives and unnecessary items. **No code changes.**

## Step 1: Read open threads

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no threads listed, report no open reviews and stop.

The output is tab-separated: `ID STATUS GOAL TITLE`

For each thread, read its contents:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <thread-id>
```

$ARGUMENTS

## Step 2: Evaluate each item

For each open thread:
1. Check if the thread has references or dependencies — if so, read those threads for context
2. Read the relevant source code (check thread anchor for file:line)
3. Assess validity — is the reviewer correct? Is the suggested change appropriate? Are concerns well-founded?

To check references and dependencies:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --refs-of <thread-id>
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --deps-of <thread-id>
```

## Step 3: Present findings

Classify each item:
- **Valid** — reviewer is right, should be addressed
- **Questionable** — may have a point but current code might be fine
- **False positive** — reviewer is wrong; explain why
- **Nitpick** — technically correct but low value

Be direct. Support assessments with specific reasoning about the code.

## Step 4: Break down complex concerns (optional)

If a thread raises multiple distinct concerns, spawn child threads to track each:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/thread-spawn.sh <parent-id> "Specific concern description"
```

Child threads inherit the parent's group and are linked via `nota-parent`.
