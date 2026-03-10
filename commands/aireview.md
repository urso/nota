---
description: Extract review comments from code and address open reviews
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
argument-hint: [optional directive e.g. "focus on auth group"]
---

# aireview — Code Review Workflow

The developer is the reviewer. Respond to their code review comments following a PR review cycle: read feedback, address each item, get confirmation before resolving.

## Step 1: Extract new comments

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/aireview.sh extract --all 2>&1`
```

If no comments found, skip to Step 2.

## Step 2: Read open reviews

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no files listed, report no open reviews and stop.

For each listed file, read open sections:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <file>
```

## Step 3: Load tag behaviors and triage

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/aireview.sh behavior 2>&1`
```

Follow the behavior description for each tag when addressing items. Present a summary:
- Item count by tag type, grouped by theme/file
- Suggested order
- Ask whether to address all or focus on specific items

$ARGUMENTS

## Step 4: Address each item

For each item:
1. Read the relevant source code
2. Follow the tag's behavior — fix, discuss, explain, implement, etc.
3. For tags requiring agreement, do NOT change code until the developer confirms

### Convergence loop

When making code changes (e.g. `impl`, `refactor`) and uncertain about the right approach, prefer leaving a new annotation over guessing. This enables iterative `/aireview` passes until all annotations converge.

To leave an annotation:
- Use the line comment syntax of the file's language (`//` for Go/C/Rust, `#` for Python/Ruby/Shell, etc.)
- Use `discuss(name):` for questions/clarifications or `propose(name):` for suggesting alternatives
- Reuse the same group name as the original annotation to keep the conversation linked

## Step 5: Resolution

After user confirmation on an item:
1. Prepend `[resolved]` or `[wontfix]` to the section heading in the tracking file
2. Add a brief response (e.g. `> Fixed: changed <= to < in validateToken()`)
3. When all sections in a file are resolved, set frontmatter `status: resolved`

Never mark an item resolved without user confirmation.
