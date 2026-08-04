package ghapi

import (
	"context"
	"fmt"
	"strconv"
)

// Review represents a GitHub PR review from REST API.
type Review struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	User        *User  `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
}

// User represents a GitHub user.
type User struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

// FetchReviews fetches all reviews for a PR via REST API.
func (c *Client) FetchReviews(ctx context.Context, repo Repo, pr int) ([]Review, error) {
	var all []Review
	page := 1

	for {
		path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews?per_page=100&page=%d",
			repo.Owner, repo.Name, pr, page)

		var reviews []Review
		headers, err := c.GetWithHeaders(ctx, path, &reviews)
		if err != nil {
			return nil, fmt.Errorf("fetching reviews: %w", err)
		}

		all = append(all, reviews...)

		if !hasNextPage(headers) {
			break
		}
		page++
	}

	return all, nil
}

func hasNextPage(headers map[string][]string) bool {
	link := headers["Link"]
	if len(link) == 0 {
		return false
	}
	for _, l := range link {
		if contains(l, `rel="next"`) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ReviewToExternalID converts a review ID to the string format used in nota.
func ReviewToExternalID(id int64) string {
	return strconv.FormatInt(id, 10)
}
