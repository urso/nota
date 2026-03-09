---
description: Critically evaluate open reviews — check if reviewer is right
allowed-tools: Bash, Read
argument-hint: [optional file or group to focus on]
---

# aireview-evaluate — Critical Review Evaluation

You are critically evaluating the developer's review comments. Your job is to assess whether each comment is valid, flag false positives, and identify items that may be wrong or unnecessary. **Do not make any code changes.**

## Step 1: Read open reviews

List open tracking files:

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no files are listed, tell the user there are no open reviews to evaluate and stop.

For each listed file, read its open sections:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

$ARGUMENTS

## Step 2: Evaluate each item

For each open review item:

1. **Read the relevant source code** around the location mentioned
2. **Assess the comment's validity:**
   - Is the reviewer correct about the issue?
   - Is this actually a problem, or a false positive?
   - Is the suggested fix appropriate, or is there a better approach?
   - For `discuss` items: are the concerns well-founded?
   - For `explain` items: is there something genuinely unclear, or is the code self-explanatory?

## Step 3: Present findings

For each item, give your assessment:

- **Valid** — the reviewer is right, this should be addressed
- **Questionable** — the reviewer may have a point but the current code might be fine; explain why
- **False positive** — the reviewer is wrong; explain why the code is correct as-is
- **Nitpick** — technically correct but low value; suggest whether it's worth addressing

Be direct and honest. The developer wants a second opinion, not agreement. Support your assessments with specific reasoning about the code.
