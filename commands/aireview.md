---
description: Extract review comments from code and address open reviews
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
argument-hint: [optional directive e.g. "focus on auth group"]
---

# aireview — Code Review Workflow

You are responding to code review comments left by the developer. The developer is the reviewer — you are the respondent. This follows a PR review workflow: the developer leaves feedback in code, you read it, and address each item.

## Step 1: Extract new comments

Run the aireview CLI to extract any new review comments from code into tracking files:

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/aireview.sh extract --all 2>&1`
```

If the tool reports no comments found, skip to Step 2.

## Step 2: Read open reviews

List open tracking files:

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no files are listed, tell the user there are no open reviews to address and stop.

For each listed file, read its open sections using the read-open script:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

Run this for every file from the list above. This filters out already-resolved sections so you only see what still needs attention.

## Step 3: Triage and plan

Classify each open review item:

| Tag | Your behavior |
|-----|--------------|
| `review` | This is a bug or issue to fix. Make the code change. |
| `discuss` | The reviewer wants to debate this. Present your perspective, propose options. Do NOT change code until the developer agrees. |
| `explain` | The reviewer wants to understand the reasoning. Explain clearly. This may turn into a discussion or code change. |

Present a summary to the user:
- How many items total, broken down by tag type
- Group by theme/file if there are many
- Suggest an order to tackle them
- Ask the user whether to address everything or focus on specific items

If the user provided a directive (e.g. "focus on auth group", "just the quick fixes"), follow it.

$ARGUMENTS

## Step 4: Address each item

Work through the items. For each one:

1. **Read the relevant source code** around the location mentioned in the review
2. **For `review` items**: Fix the issue. Show what you changed.
3. **For `discuss` items**: Present your analysis and options. Wait for the developer's input before making changes.
4. **For `explain` items**: Provide a clear explanation. Ask if the developer is satisfied or wants changes.

## Step 5: Resolution

After addressing an item and getting user confirmation:

1. Edit the tracking file in `.aireview/`
2. Prepend `[resolved]` or `[wontfix]` to the section heading
3. Add a brief response comment below the heading (e.g. `> Fixed: changed <= to < in validateToken()`)
4. When ALL sections in a file are resolved, update the frontmatter `status: resolved`

**IMPORTANT: Never mark an item resolved without user confirmation.** Always ask: "Want me to mark this resolved?" or wait for the user to say it's good.

## Rules

- You are the respondent, not the reviewer. Respect the developer's feedback.
- You can push back if you think the reviewer is wrong, but explain why.
- For `discuss` items, never change code without agreement.
- If a review comment references code that has changed significantly, note this to the user.
- Work incrementally — don't try to resolve everything in one message unless the user asks for it.
