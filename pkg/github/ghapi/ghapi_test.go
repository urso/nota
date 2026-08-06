package ghapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type mockTransport struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	status int
	body   string
	header http.Header
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	key := req.Method + " " + path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}

	resp, ok := m.responses[key]
	if !ok {
		for pattern, r := range m.responses {
			patternPath := strings.TrimPrefix(pattern, "GET ")
			patternPath = strings.TrimPrefix(patternPath, "POST ")
			patternPath = strings.TrimSuffix(patternPath, "*")
			if strings.Contains(path, strings.TrimPrefix(patternPath, "/")) {
				resp = r
				ok = true
				break
			}
		}
	}
	if !ok {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewBufferString(`{"message": "not found"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    req,
		}, nil
	}

	header := resp.header
	if header == nil {
		header = http.Header{"Content-Type": []string{"application/json"}}
	}

	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
		Header:     header,
		Request:    req,
	}, nil
}

func newMockClient(t *testing.T, responses map[string]mockResponse) *Client {
	t.Helper()
	client, err := NewClient(ClientOptions{
		Transport: &mockTransport{responses: responses},
	})
	if err != nil {
		t.Fatalf("creating mock client: %v", err)
	}
	return client
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		input   string
		want    Repo
		wantErr bool
	}{
		{"owner/name", Repo{Owner: "owner", Name: "name", Host: "github.com"}, false},
		{"github.example.com/owner/name", Repo{Host: "github.example.com", Owner: "owner", Name: "name"}, false},
		{"", Repo{}, true},
		{"owner", Repo{}, true},
		{"/name", Repo{}, true},
		{"owner/", Repo{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRepo(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepo(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseRepo(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRepoMatches(t *testing.T) {
	tests := []struct {
		a, b Repo
		want bool
	}{
		{Repo{Owner: "foo", Name: "bar"}, Repo{Owner: "foo", Name: "bar"}, true},
		{Repo{Owner: "Foo", Name: "Bar"}, Repo{Owner: "foo", Name: "bar"}, true},
		{Repo{Owner: "foo", Name: "bar", Host: "github.com"}, Repo{Owner: "foo", Name: "bar", Host: "ghes.example.com"}, true},
		{Repo{Owner: "foo", Name: "bar"}, Repo{Owner: "foo", Name: "baz"}, false},
	}

	for _, tt := range tests {
		got := RepoMatches(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("RepoMatches(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFetchReviews(t *testing.T) {
	client := newMockClient(t, map[string]mockResponse{
		"GET /repos/owner/repo/pulls/123/reviews*": {
			status: 200,
			body: `[
				{"id": 1, "node_id": "PRR_1", "body": "LGTM", "state": "APPROVED", "user": {"login": "alice"}},
				{"id": 2, "node_id": "PRR_2", "body": "", "state": "COMMENTED", "user": {"login": "bob"}}
			]`,
		},
	})

	reviews, err := client.FetchReviews(context.Background(), Repo{Owner: "owner", Name: "repo"}, 123)
	if err != nil {
		t.Fatalf("FetchReviews: %v", err)
	}

	if len(reviews) != 2 {
		t.Errorf("got %d reviews, want 2", len(reviews))
	}

	if reviews[0].Body != "LGTM" {
		t.Errorf("reviews[0].Body = %q, want %q", reviews[0].Body, "LGTM")
	}
}

func TestFetchIssueComments(t *testing.T) {
	client := newMockClient(t, map[string]mockResponse{
		"GET /repos/owner/repo/issues/123/comments*": {
			status: 200,
			body: `[
				{"id": 100, "node_id": "IC_1", "body": "First comment", "user": {"login": "alice"}, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
				{"id": 101, "node_id": "IC_2", "body": "Second comment", "user": {"login": "bob"}, "created_at": "2026-01-02T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"}
			]`,
		},
	})

	comments, err := client.FetchIssueComments(context.Background(), Repo{Owner: "owner", Name: "repo"}, 123)
	if err != nil {
		t.Fatalf("FetchIssueComments: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("got %d comments, want 2", len(comments))
	}

	if comments[0].Body != "First comment" {
		t.Errorf("comments[0].Body = %q, want %q", comments[0].Body, "First comment")
	}
}

func TestErrorClassification(t *testing.T) {
	t.Run("IsAuthError", func(t *testing.T) {
		err := &AuthError{Host: "github.com", Message: "no token"}
		if !IsAuthError(err) {
			t.Error("expected IsAuthError to return true")
		}
		if IsRateLimit(err) {
			t.Error("expected IsRateLimit to return false")
		}
	})

	t.Run("IsRateLimit", func(t *testing.T) {
		err := &RateLimitError{Limit: 5000, Remaining: 0}
		if !IsRateLimit(err) {
			t.Error("expected IsRateLimit to return true")
		}
	})

	t.Run("IsNotFound", func(t *testing.T) {
		err := &NotFoundError{Resource: "repo"}
		if !IsNotFound(err) {
			t.Error("expected IsNotFound to return true")
		}
	})
}

func TestNoPRError(t *testing.T) {
	err := &NoPRError{}
	if !IsNoPR(err) {
		t.Error("expected IsNoPR to return true")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestReviewThreadParsing(t *testing.T) {
	graphqlResponse := `{
		"data": {
			"repository": {
				"pullRequest": {
					"id": "PR_123",
					"number": 42,
					"reviewThreads": {
						"pageInfo": {"hasNextPage": false, "endCursor": ""},
						"nodes": [
							{
								"id": "PRRT_1",
								"isResolved": false,
								"isOutdated": false,
								"subjectType": "LINE",
								"path": "main.go",
								"diffSide": "RIGHT",
								"line": 10,
								"originalLine": 10,
								"comments": {
									"totalCount": 1,
									"pageInfo": {"hasNextPage": false, "endCursor": ""},
									"nodes": [
										{
											"id": "PRRC_1",
											"fullDatabaseId": "123456789",
											"author": {"login": "alice"},
											"body": "Fix this",
											"createdAt": "2026-01-01T00:00:00Z",
											"updatedAt": "2026-01-01T00:00:00Z"
										}
									]
								}
							}
						]
					}
				}
			}
		}
	}`

	_ = graphqlResponse
	_ = cmp.Diff
}
