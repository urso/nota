---
description: Pull GitHub PR comments into nota threads for the current branch
allowed-tools: Bash, Read
---

# nota-gh-pull — Import GitHub PR Comments

Pull review comments from the current branch's PR into local nota threads.

## Prerequisites

- `gh` CLI installed and authenticated (`gh auth login`)
- Current branch has an associated PR

## Step 1: Pull the PR

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/sync-pull.sh 2>&1
```

If the command fails with an auth error, instruct the user to run `gh auth login`.

If no PR is associated with the current branch, tell the user to push their branch and open a PR first, or checkout a branch that has one.

## Step 2: Show the imported threads

After a successful pull, list the imported threads:

```bash
bash ${CLAUDE_PLUGIN_ROOT}/scripts/list-open.sh 2>&1
```

Present a summary:
- Total threads pulled (inline + conversation)
- Resolved vs open count
- Any threads with `goal="review"` that need attention

