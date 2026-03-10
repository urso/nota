---
description: Critically evaluate open reviews — check if reviewer is right
allowed-tools: Bash, Read
argument-hint: [optional file or group to focus on]
---

# aireview-evaluate — Critical Review Evaluation

Critically assess whether each review comment is valid. Flag false positives and unnecessary items. **No code changes.**

## Step 1: Read open reviews

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no files listed, report no open reviews and stop.

For each listed file, read open sections:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

$ARGUMENTS

## Step 2: Evaluate each item

For each open item:
1. Read the relevant source code
2. Assess validity — is the reviewer correct? Is the suggested change appropriate? Are concerns well-founded?

## Step 3: Present findings

Classify each item:
- **Valid** — reviewer is right, should be addressed
- **Questionable** — may have a point but current code might be fine
- **False positive** — reviewer is wrong; explain why
- **Nitpick** — technically correct but low value

Be direct. Support assessments with specific reasoning about the code.
