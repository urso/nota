#!/bin/bash
# Capture real GitHub API responses as test fixtures.
# Requires: gh auth login, jq
#
# Usage: ./capture-fixtures.sh
#
# Targets public PRs known to have the edge cases we need:
# - cli/cli#13541: mature PR with outdated threads, multi-comment threads
# - kubernetes/kubernetes#140512: has file-level comments

set -euo pipefail
cd "$(dirname "$0")"

echo "Checking prerequisites..."
command -v gh >/dev/null || { echo "gh CLI not found"; exit 1; }
command -v jq >/dev/null || { echo "jq not found"; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "gh not authenticated"; exit 1; }

# GraphQL query for review threads (same as threads.go)
read -r -d '' THREADS_QUERY << 'EOF' || true
query($owner: String!, $name: String!, $pr: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $pr) {
      id
      number
      reviewThreads(first: 50) {
        nodes {
          id
          isResolved
          isOutdated
          subjectType
          path
          diffSide
          line
          originalLine
          startLine
          comments(first: 10) {
            totalCount
            nodes {
              id
              fullDatabaseId
              author { login }
              body
              createdAt
              updatedAt
              lastEditedAt
              commit { oid }
              originalCommit { oid }
            }
          }
        }
      }
    }
  }
}
EOF

capture_threads() {
    local owner=$1 repo=$2 pr=$3 outfile=$4
    echo "Fetching threads from $owner/$repo#$pr..."
    gh api graphql -f query="$THREADS_QUERY" \
        -F owner="$owner" -F name="$repo" -F pr="$pr" \
        | jq '.data.repository.pullRequest.reviewThreads.nodes' \
        > "$outfile"
    echo "  -> $outfile ($(jq length "$outfile") threads)"
}

capture_reviews() {
    local owner=$1 repo=$2 pr=$3 outfile=$4
    echo "Fetching reviews from $owner/$repo#$pr..."
    gh api "repos/$owner/$repo/pulls/$pr/reviews" --paginate \
        | jq '[.[] | {id, node_id, user: .user.login, body, state, submitted_at}]' \
        > "$outfile"
    echo "  -> $outfile ($(jq length "$outfile") reviews)"
}

capture_issue_comments() {
    local owner=$1 repo=$2 pr=$3 outfile=$4
    echo "Fetching issue comments from $owner/$repo#$pr..."
    gh api "repos/$owner/$repo/issues/$pr/comments" --paginate \
        | jq '[.[] | {id, node_id, user: .user.login, body, created_at, updated_at}]' \
        > "$outfile"
    echo "  -> $outfile ($(jq length "$outfile") comments)"
}

# Find specific fixture cases
find_outdated_thread() {
    local file=$1
    jq 'map(select(.isOutdated == true and .line == null)) | .[0] // empty' "$file"
}

find_file_comment() {
    local file=$1
    jq 'map(select(.subjectType == "FILE")) | .[0] // empty' "$file"
}

find_multi_comment_thread() {
    local file=$1
    jq 'map(select(.comments.totalCount > 1)) | .[0] // empty' "$file"
}

find_updated_comment() {
    local file=$1
    jq '[.[] | .comments.nodes[] | select(.updatedAt != .createdAt and .lastEditedAt == null)] | .[0] // empty' "$file"
}

find_empty_review() {
    local file=$1
    jq 'map(select(.body == "")) | .[0] // empty' "$file"
}

echo ""
echo "=== Capturing from cli/cli#13541 (mature PR with outdated threads) ==="
capture_threads "cli" "cli" 13541 "_raw_threads_cli.json"
capture_reviews "cli" "cli" 13541 "_raw_reviews_cli.json"
capture_issue_comments "cli" "cli" 13541 "_raw_comments_cli.json"

echo ""
echo "=== Capturing from kubernetes/kubernetes#140512 (has file-level comments) ==="
capture_threads "kubernetes" "kubernetes" 140512 "_raw_threads_k8s.json"

echo ""
echo "=== Extracting specific fixtures ==="

# Outdated thread
echo -n "Outdated thread: "
find_outdated_thread "_raw_threads_cli.json" > thread_outdated.json
if [ -s thread_outdated.json ]; then
    echo "found (line=$(jq '.line' thread_outdated.json), originalLine=$(jq '.originalLine' thread_outdated.json))"
else
    echo "NOT FOUND"
fi

# File-level comment
echo -n "File-level comment: "
find_file_comment "_raw_threads_k8s.json" > thread_file_level.json
if [ -s thread_file_level.json ]; then
    echo "found (subjectType=$(jq -r '.subjectType' thread_file_level.json), line=$(jq '.line' thread_file_level.json))"
else
    # Try cli repo as fallback
    find_file_comment "_raw_threads_cli.json" > thread_file_level.json
    if [ -s thread_file_level.json ]; then
        echo "found in cli (subjectType=$(jq -r '.subjectType' thread_file_level.json))"
    else
        echo "NOT FOUND"
    fi
fi

# Multi-comment thread
echo -n "Multi-comment thread: "
find_multi_comment_thread "_raw_threads_cli.json" > thread_multi_comment.json
if [ -s thread_multi_comment.json ]; then
    echo "found ($(jq '.comments.totalCount' thread_multi_comment.json) comments)"
else
    echo "NOT FOUND"
fi

# Updated comment (updatedAt != createdAt, lastEditedAt null)
echo -n "Updated comment (non-edit bump): "
find_updated_comment "_raw_threads_cli.json" > comment_updated.json
if [ -s comment_updated.json ]; then
    echo "found (created=$(jq -r '.createdAt' comment_updated.json | cut -c12-19), updated=$(jq -r '.updatedAt' comment_updated.json | cut -c12-19))"
else
    echo "NOT FOUND"
fi

# Empty review summary
echo -n "Empty review summary: "
find_empty_review "_raw_reviews_cli.json" > review_empty.json
if [ -s review_empty.json ]; then
    echo "found (state=$(jq -r '.state' review_empty.json))"
else
    echo "NOT FOUND"
fi

echo ""
echo "=== Cleanup ==="
rm -f _raw_*.json
echo "Removed raw files, kept trimmed fixtures."

echo ""
echo "=== Summary ==="
ls -la *.json 2>/dev/null || echo "No fixtures captured"
