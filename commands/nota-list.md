---
description: List review threads with optional filters
allowed-tools: Bash
argument-hint: [optional filters e.g. "--status=open" or "--goal=review"]
---

# nota-list — List Review Threads

List threads from `.nota/` with optional filtering.

```
!`bash ${CLAUDE_PLUGIN_ROOT}/scripts/nota.sh thread list $ARGUMENTS 2>&1`
```

The output is tab-separated: `ID STATUS GOAL TITLE`

Available filters:
- `--status=open|resolved|wontfix` — filter by thread status
- `--goal=review|discuss|impl|...` — filter by goal
- `--group=<name>` — filter by group name
- `--tag=<tag>` — filter by tag

Relationship queries:
- `--refs-of=<id>` — list threads that the given thread references
- `--deps-of=<id>` — list threads that the given thread depends on
- `--referenced-by=<id>` — list threads that reference the given thread
- `--blocked-by=<id>` — list threads that depend on the given thread

Present the output to the user as-is.
