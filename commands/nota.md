---
description: Extract review comments from code and address open reviews
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
argument-hint: [optional directive e.g. "focus on auth group"]
---

# nota — Code Review Workflow

The developer is the reviewer. Respond to their code review comments following a PR review cycle: read feedback, address each item, get confirmation before resolving.

## Step 1: Extract new comments

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/extract.sh --all 2>&1`
```

If no comments found, skip to Step 2.

## Step 2: Read open threads

```
`bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh`
```

If no threads listed, report no open reviews and stop.

The output is tab-separated: `ID STATUS GOAL TITLE`

For each thread, read its contents:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/read-open.sh <thread-id>
```

## Step 3: Load tag behaviors and triage

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh behavior 2>&1`
```

Follow the behavior description for each goal when addressing items. Present a summary:
- Item count by goal type, grouped by theme/group
- Dependency relationships (threads with `depends-on`)
- Suggested order (items with unsatisfied dependencies should be addressed later)
- Ask whether to address all or focus on specific items

$ARGUMENTS

## Step 4: Gather context and address each item

Before addressing a thread, check if it has references or dependencies. If so, read those threads for context — they may include resolved discussions with relevant decisions.

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --refs-of <thread-id>
bash ${CLAUDE_PLUGIN_ROOT}/scripts/find-review.sh --deps-of <thread-id>
```

For each thread:
1. Read referenced/dependent threads for context
2. Read the relevant source code (check thread anchor for file:line)
3. Follow the goal's behavior — fix, discuss, explain, implement, etc.
4. For goals requiring agreement, do NOT change code until the developer confirms

### Convergence loop

When making code changes (e.g. `impl`, `refactor`) and uncertain about the right approach, prefer leaving a new annotation over guessing. This enables iterative `/nota` passes until all annotations converge.

To leave an annotation:
- Use the line comment syntax of the file's language (`//` for Go/C/Rust, `#` for Python/Ruby/Shell, etc.)
- Use `discuss(name):` for questions/clarifications or `propose(name):` for suggesting alternatives
- Reuse the same group name as the original annotation to keep the conversation linked

### Responding to threads

Add your response as a comment to the thread:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/thread-add.sh <thread-id> "Your response message"
```

For longer responses, pipe from stdin:

```bash
echo "Your response" | bash ${CLAUDE_PLUGIN_ROOT}/scripts/thread-add.sh <thread-id> --file=-
```

### Authorship

A thread is a conversation between the developer and you, so each comment must
be attributed to whoever actually wrote it. `thread-add.sh` and
`thread-spawn.sh` record `author="agent"` by default — do not pass a flag for
your own comments.

Only when you are transcribing something the developer said (relaying their
decision verbatim, not summarizing it) add `--human`, which records the comment
under their git user instead:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/thread-add.sh --human <thread-id> "Their words"
```

Never record your own analysis, questions, or conclusions as `--human` — that
puts your words under the developer's name and breaks the conversation.

## Step 5: Resolution

After user confirmation on an item:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/thread-resolve.sh <thread-id>
```

**Important**:
- Never mark a thread resolved without user confirmation.
- Never delete `.nota/` files — they are persistent records used for external sync. Use the resolve command to set status; deletion is a separate explicit action via `/nota-cleanup`.
