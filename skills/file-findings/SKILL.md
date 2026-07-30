---
description: "Use when you have review findings (bugs, issues, style problems) that cannot be addressed in the current session. Creates local tickets for later work or handoff to another agent."
version: 1.0.0
user-invocable: true
---

# File Findings — Create Tickets from Review Results

When a code review produces findings too complex or numerous to address in one session, file them as nota threads for later work.

## When to use

- Review produced multiple findings requiring separate investigation
- Complex bug requires dedicated session with full context
- Findings span multiple subsystems needing coordinated work
- Handoff to another agent or future session

## Creating a ticket

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh thread create "<title>" \
  --agent \
  --goal=review \
  --anchor="<file>:<line>" \
  --tags="<severity>,<category>" \
  --body="<details>"
```

### Fields

- **title**: One-line summary of the finding
- **agent**: Always pass `--agent`. You are filing this finding, so it must be
  attributed to `author="agent"` and not to the developer's git user.
- **goal**: Use `review`
- **anchor**: File and line number (e.g. `internal/txn.go:91`)
- **tags**: Comma-separated labels. Conventions:
  - Severity: `severity:critical`, `severity:warning`, `severity:info`
  - Category: `category:correctness`, `category:efficiency`, `category:test-coverage`, `category:style`
- **body**: Detailed description including trigger scenario, impact, suggested fix

### Example

From a finding:

> **B1. Race condition between Done() and Cleanup()**
> File: internal/volumetxn/txn.go:91-131
> Both check and set t.done without synchronization...

Create:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh thread create "Race condition: Done() and Cleanup() unsynchronized access to t.done" \
  --agent \
  --goal=review \
  --anchor="internal/volumetxn/txn.go:91" \
  --tags="severity:critical,category:correctness" \
  --body="[bug] Both methods check and set t.done without synchronization.

Trigger scenario:
- t0: goroutine A calls Done(), reads t.done == false
- t1: goroutine B calls Cleanup(), reads t.done == false
- t2: A sets t.done = true, nils cleanups, calls releaseAll()
- t3: B sets t.done = true, captures cleanups (now nil)
- t4: B calls releaseAll() AGAIN on released locks -> panic

Suggested fix: Add sync.Mutex to protect t.done, t.cleanups, t.locks, or use sync.Once for release."
```

## Reading body from stdin

For long bodies, use `--body=-`:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh thread create "Title" --agent --goal=review --anchor="file:line" --body=- <<'EOF'
[bug] Description here...

## Trigger scenario
...

## Suggested fix
...
EOF
```
