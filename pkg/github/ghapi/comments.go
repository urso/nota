package ghapi

import (
	"context"
	"fmt"
	"strconv"
)

// IssueComment represents a GitHub issue/PR conversation comment from REST API.
type IssueComment struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	User      *User  `json:"user"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FetchIssueComments fetches all issue comments for a PR via REST API.
// These are the PR conversation comments, not inline review comments.
func (c *Client) FetchIssueComments(ctx context.Context, repo Repo, pr int) ([]IssueComment, error) {
	var all []IssueComment
	page := 1

	for {
		path := fmt.Sprintf("repos/%s/%s/issues/%d/comments?per_page=100&page=%d",
			repo.Owner, repo.Name, pr, page)

		var comments []IssueComment
		headers, err := c.GetWithHeaders(ctx, path, &comments)
		if err != nil {
			return nil, fmt.Errorf("fetching issue comments: %w", err)
		}

		all = append(all, comments...)

		if !hasNextPage(headers) {
			break
		}
		page++
	}

	return all, nil
}

// IssueCommentToExternalID converts an issue comment ID to the string format used in nota.
func IssueCommentToExternalID(id int64) string {
	return strconv.FormatInt(id, 10)
}
