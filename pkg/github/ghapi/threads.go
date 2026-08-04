package ghapi

import (
	"context"
	"fmt"
)

// ReviewThread represents a GitHub PR review thread from GraphQL.
type ReviewThread struct {
	ID          string // PRRT_ node ID
	IsResolved  bool
	IsOutdated  bool
	SubjectType string // "LINE" or "FILE"
	Path        string
	DiffSide    string // "LEFT" or "RIGHT"

	Line         *int
	OriginalLine *int
	StartLine    *int

	CommitID         string
	OriginalCommitID string

	Comments []ReviewComment
}

// ReviewComment represents a comment within a review thread.
type ReviewComment struct {
	ID             string // node ID (PRRC_...)
	FullDatabaseID string // numeric ID as string
	Author         string // login or empty for ghost
	Body           string
	CreatedAt      string
	UpdatedAt      string
	LastEditedAt   *string
}

// PRInfo holds PR-level metadata from GraphQL.
type PRInfo struct {
	ID     string // PR node ID (PR_...)
	Number int
}

const reviewThreadsQuery = `
query($owner: String!, $name: String!, $pr: Int!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $pr) {
      id
      number
      reviewThreads(first: 100, after: $cursor) {
        pageInfo {
          hasNextPage
          endCursor
        }
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
          comments(first: 100) {
            totalCount
            pageInfo {
              hasNextPage
              endCursor
            }
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
`

const threadCommentsQuery = `
query($threadId: ID!, $cursor: String!) {
  node(id: $threadId) {
    ... on PullRequestReviewThread {
      comments(first: 100, after: $cursor) {
        totalCount
        pageInfo {
          hasNextPage
          endCursor
        }
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
`

type reviewThreadsResponse struct {
	Repository struct {
		PullRequest struct {
			ID            string
			Number        int
			ReviewThreads struct {
				PageInfo pageInfo
				Nodes    []reviewThreadNode
			}
		}
	}
}

type reviewThreadNode struct {
	ID           string
	IsResolved   bool
	IsOutdated   bool
	SubjectType  string
	Path         string
	DiffSide     string
	Line         *int
	OriginalLine *int
	StartLine    *int
	Comments     commentsConnection
}

type commentsConnection struct {
	TotalCount int
	PageInfo   pageInfo
	Nodes      []commentNode
}

type commentNode struct {
	ID             string
	FullDatabaseId string
	Author         *struct{ Login string }
	Body           string
	CreatedAt      string
	UpdatedAt      string
	LastEditedAt   *string
	Commit         *struct{ Oid string }
	OriginalCommit *struct{ Oid string }
}

type pageInfo struct {
	HasNextPage bool
	EndCursor   string
}

type threadCommentsResponse struct {
	Node struct {
		Comments commentsConnection
	}
}

// FetchReviewThreads fetches all review threads for a PR, with complete comment pagination.
func (c *Client) FetchReviewThreads(ctx context.Context, repo Repo, pr int) ([]ReviewThread, PRInfo, error) {
	var threads []ReviewThread
	var prInfo PRInfo
	var cursor *string

	for {
		vars := map[string]any{
			"owner":  repo.Owner,
			"name":   repo.Name,
			"pr":     pr,
			"cursor": cursor,
		}

		var resp reviewThreadsResponse
		if err := c.Query(ctx, reviewThreadsQuery, vars, &resp); err != nil {
			return nil, PRInfo{}, fmt.Errorf("fetching review threads: %w", err)
		}

		prData := resp.Repository.PullRequest
		if prInfo.ID == "" {
			prInfo = PRInfo{ID: prData.ID, Number: prData.Number}
		}

		for _, node := range prData.ReviewThreads.Nodes {
			thread, err := c.fetchCompleteThread(ctx, node)
			if err != nil {
				return nil, PRInfo{}, err
			}
			threads = append(threads, thread)
		}

		if !prData.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		cursor = &prData.ReviewThreads.PageInfo.EndCursor
	}

	return threads, prInfo, nil
}

func (c *Client) fetchCompleteThread(ctx context.Context, node reviewThreadNode) (ReviewThread, error) {
	thread := ReviewThread{
		ID:           node.ID,
		IsResolved:   node.IsResolved,
		IsOutdated:   node.IsOutdated,
		SubjectType:  node.SubjectType,
		Path:         node.Path,
		DiffSide:     node.DiffSide,
		Line:         node.Line,
		OriginalLine: node.OriginalLine,
		StartLine:    node.StartLine,
	}

	comments := convertComments(node.Comments.Nodes)
	if len(comments) > 0 {
		if c := node.Comments.Nodes[0]; c.Commit != nil {
			thread.CommitID = c.Commit.Oid
		}
		if c := node.Comments.Nodes[0]; c.OriginalCommit != nil {
			thread.OriginalCommitID = c.OriginalCommit.Oid
		}
	}

	if node.Comments.PageInfo.HasNextPage {
		more, err := c.fetchRemainingComments(ctx, node.ID, node.Comments.PageInfo.EndCursor, node.Comments.TotalCount-len(comments))
		if err != nil {
			return ReviewThread{}, err
		}
		comments = append(comments, more...)
	}

	if len(comments) != node.Comments.TotalCount {
		return ReviewThread{}, fmt.Errorf("comment count mismatch for thread %s: got %d, expected %d",
			node.ID, len(comments), node.Comments.TotalCount)
	}

	thread.Comments = comments
	return thread, nil
}

func (c *Client) fetchRemainingComments(ctx context.Context, threadID, cursor string, _ int) ([]ReviewComment, error) {
	var comments []ReviewComment

	for {
		vars := map[string]any{
			"threadId": threadID,
			"cursor":   cursor,
		}

		var resp threadCommentsResponse
		if err := c.Query(ctx, threadCommentsQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("fetching comments for thread %s: %w", threadID, err)
		}

		comments = append(comments, convertComments(resp.Node.Comments.Nodes)...)

		if !resp.Node.Comments.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Node.Comments.PageInfo.EndCursor
	}

	return comments, nil
}

func convertComments(nodes []commentNode) []ReviewComment {
	comments := make([]ReviewComment, 0, len(nodes))
	for _, n := range nodes {
		c := ReviewComment{
			ID:             n.ID,
			FullDatabaseID: n.FullDatabaseId,
			Body:           n.Body,
			CreatedAt:      n.CreatedAt,
			UpdatedAt:      n.UpdatedAt,
			LastEditedAt:   n.LastEditedAt,
		}
		if n.Author != nil {
			c.Author = n.Author.Login
		}
		comments = append(comments, c)
	}
	return comments
}
