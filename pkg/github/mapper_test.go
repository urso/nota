package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/urso/nota/pkg/github/ghapi"
	"github.com/urso/nota/pkg/thread"
)

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("ghapi", "testdata", name))
	if err != nil {
		t.Fatalf("loading fixture %s: %v", name, err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	return v
}

func TestMapReviewThread_FileLevelComment(t *testing.T) {
	raw := loadFixture[json.RawMessage](t, "thread_file_level.json")

	var node struct {
		ID           string
		IsResolved   bool
		IsOutdated   bool
		SubjectType  string
		Path         string
		DiffSide     string
		Line         *int
		OriginalLine *int
		StartLine    *int
		Comments     struct {
			TotalCount int
			Nodes      []struct {
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
		}
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("parsing node: %v", err)
	}

	rt := ghapi.ReviewThread{
		ID:          node.ID,
		IsResolved:  node.IsResolved,
		IsOutdated:  node.IsOutdated,
		SubjectType: node.SubjectType,
		Path:        node.Path,
		DiffSide:    node.DiffSide,
		Line:        node.Line,
	}
	if len(node.Comments.Nodes) > 0 {
		c := node.Comments.Nodes[0]
		if c.Commit != nil {
			rt.CommitID = c.Commit.Oid
		}
		if c.OriginalCommit != nil {
			rt.OriginalCommitID = c.OriginalCommit.Oid
		}
		author := ""
		if c.Author != nil {
			author = c.Author.Login
		}
		rt.Comments = []ghapi.ReviewComment{{
			ID:             c.ID,
			FullDatabaseID: c.FullDatabaseId,
			Author:         author,
			Body:           c.Body,
			CreatedAt:      c.CreatedAt,
			UpdatedAt:      c.UpdatedAt,
		}}
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 140512}
	resolvedRepo := ghapi.Repo{Owner: "kubernetes", Name: "kubernetes"}

	th := mapReviewThread(rt, prInfo, resolvedRepo)

	if th.CurrentAnchor() != nil {
		t.Errorf("file-level comment should produce nil CurrentAnchor(), got %+v", th.CurrentAnchor())
	}

	if len(th.FileAnchors) != 1 {
		t.Fatalf("expected 1 FileAnchor, got %d", len(th.FileAnchors))
	}

	fa := th.FileAnchors[0]
	if fa.File != node.Path {
		t.Errorf("FileAnchor.File = %q, want %q", fa.File, node.Path)
	}

	if len(th.Anchors) != 0 {
		t.Errorf("file-level comment should have no line Anchors, got %d", len(th.Anchors))
	}

	if th.Goal != "review" {
		t.Errorf("Goal = %q, want %q", th.Goal, "review")
	}

	if th.Sync.Kind != "" {
		t.Errorf("review-thread should have empty Kind (defaults to review-thread), got %q", th.Sync.Kind)
	}
}

func TestMapReviewThread_OutdatedComment(t *testing.T) {
	raw := loadFixture[json.RawMessage](t, "thread_outdated.json")

	var node struct {
		ID           string
		IsResolved   bool
		IsOutdated   bool
		SubjectType  string
		Path         string
		DiffSide     string
		Line         *int
		OriginalLine *int
		StartLine    *int
		Comments     struct {
			TotalCount int
			Nodes      []struct {
				ID             string
				FullDatabaseId string
				Author         *struct{ Login string }
				Body           string
				CreatedAt      string
				UpdatedAt      string
				Commit         *struct{ Oid string }
				OriginalCommit *struct{ Oid string }
			}
		}
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("parsing node: %v", err)
	}

	rt := ghapi.ReviewThread{
		ID:           node.ID,
		IsResolved:   node.IsResolved,
		IsOutdated:   node.IsOutdated,
		SubjectType:  node.SubjectType,
		Path:         node.Path,
		DiffSide:     node.DiffSide,
		Line:         node.Line,
		OriginalLine: node.OriginalLine,
	}
	if len(node.Comments.Nodes) > 0 {
		c := node.Comments.Nodes[0]
		if c.Commit != nil {
			rt.CommitID = c.Commit.Oid
		}
		if c.OriginalCommit != nil {
			rt.OriginalCommitID = c.OriginalCommit.Oid
		}
		author := ""
		if c.Author != nil {
			author = c.Author.Login
		}
		rt.Comments = []ghapi.ReviewComment{{
			ID:             c.ID,
			FullDatabaseID: c.FullDatabaseId,
			Author:         author,
			Body:           c.Body,
			CreatedAt:      c.CreatedAt,
			UpdatedAt:      c.UpdatedAt,
		}}
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 13541}
	resolvedRepo := ghapi.Repo{Owner: "cli", Name: "cli"}

	th := mapReviewThread(rt, prInfo, resolvedRepo)

	anchor := th.CurrentAnchor()
	if anchor == nil {
		t.Fatal("outdated comment should have a line anchor")
	}

	if !anchor.Outdated {
		t.Error("anchor.Outdated should be true for line==null case")
	}

	if anchor.Line != 371 {
		t.Errorf("anchor.Line = %d, want 371 (from originalLine)", anchor.Line)
	}

	if anchor.Commit != rt.OriginalCommitID {
		t.Errorf("anchor.Commit = %q, want %q (originalCommit)", anchor.Commit, rt.OriginalCommitID)
	}

	if th.Status != "resolved" {
		t.Errorf("Status = %q, want %q (from isResolved=true)", th.Status, "resolved")
	}
}

func TestMapReviewThread_MultiComment(t *testing.T) {
	raw := loadFixture[json.RawMessage](t, "thread_multi_comment.json")

	var node struct {
		ID          string
		IsResolved  bool
		SubjectType string
		Path        string
		Comments    struct {
			TotalCount int
			Nodes      []struct {
				ID             string
				FullDatabaseId string
				Author         *struct{ Login string }
				Body           string
				CreatedAt      string
				UpdatedAt      string
				Commit         *struct{ Oid string }
				OriginalCommit *struct{ Oid string }
			}
		}
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("parsing node: %v", err)
	}

	var comments []ghapi.ReviewComment
	var commitID, origCommitID string
	for i, c := range node.Comments.Nodes {
		if i == 0 {
			if c.Commit != nil {
				commitID = c.Commit.Oid
			}
			if c.OriginalCommit != nil {
				origCommitID = c.OriginalCommit.Oid
			}
		}
		author := ""
		if c.Author != nil {
			author = c.Author.Login
		}
		comments = append(comments, ghapi.ReviewComment{
			ID:             c.ID,
			FullDatabaseID: c.FullDatabaseId,
			Author:         author,
			Body:           c.Body,
			CreatedAt:      c.CreatedAt,
			UpdatedAt:      c.UpdatedAt,
		})
	}

	rt := ghapi.ReviewThread{
		ID:               node.ID,
		IsResolved:       node.IsResolved,
		SubjectType:      node.SubjectType,
		Path:             node.Path,
		CommitID:         commitID,
		OriginalCommitID: origCommitID,
		Comments:         comments,
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 13541}
	resolvedRepo := ghapi.Repo{Owner: "cli", Name: "cli"}

	th := mapReviewThread(rt, prInfo, resolvedRepo)

	if len(th.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(th.Comments))
	}

	for i, c := range th.Comments {
		if c.Anchor != nil {
			t.Errorf("comment[%d].Anchor should be nil on pulled comments", i)
		}
		if c.ExternalID == "" {
			t.Errorf("comment[%d].ExternalID should be set", i)
		}
		if c.UpdatedAt == "" {
			t.Errorf("comment[%d].UpdatedAt should be set", i)
		}
		if c.SyncStatus != "pulled" {
			t.Errorf("comment[%d].SyncStatus = %q, want %q", i, c.SyncStatus, "pulled")
		}
		if len(c.Bodies) == 0 || c.Bodies[0].Time == "" {
			t.Errorf("comment[%d].Bodies[0].Time should be non-empty", i)
		}
	}
}

func TestMapReviewComment_AuthorMapping(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		wantAuthor string
	}{
		{"normal user", "alice", "github:alice"},
		{"bot user", "dependabot[bot]", "github:dependabot[bot]"},
		{"ghost (empty)", "", "github:ghost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := ghapi.ReviewComment{
				ID:             "PRRC_test",
				FullDatabaseID: "12345",
				Author:         tt.author,
				Body:           "test",
				CreatedAt:      "2026-01-01T00:00:00Z",
				UpdatedAt:      "2026-01-01T00:00:00Z",
			}

			c := mapReviewComment(rc)

			if c.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", c.Author, tt.wantAuthor)
			}
		})
	}
}

func TestMapConversationThread_EmptyReviewsFiltered(t *testing.T) {
	reviews := []ghapi.Review{
		{ID: 1, NodeID: "PRR_1", Body: "", State: "APPROVED", SubmittedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, NodeID: "PRR_2", Body: "LGTM", State: "APPROVED", SubmittedAt: "2026-01-02T00:00:00Z", User: &ghapi.User{Login: "alice"}},
		{ID: 3, NodeID: "PRR_3", Body: "", State: "COMMENTED", SubmittedAt: "2026-01-03T00:00:00Z"},
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 123}
	resolvedRepo := ghapi.Repo{Owner: "owner", Name: "repo"}

	th := mapConversationThread(reviews, nil, prInfo, resolvedRepo)

	if th == nil {
		t.Fatal("expected non-nil thread")
	}

	if len(th.Comments) != 1 {
		t.Fatalf("expected 1 comment (empty bodies filtered), got %d", len(th.Comments))
	}

	if th.Comments[0].Author != "github:alice" {
		t.Errorf("comment Author = %q, want %q", th.Comments[0].Author, "github:alice")
	}
}

func TestMapConversationThread_NoComments(t *testing.T) {
	reviews := []ghapi.Review{
		{ID: 1, NodeID: "PRR_1", Body: "", State: "APPROVED"},
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 123}
	resolvedRepo := ghapi.Repo{Owner: "owner", Name: "repo"}

	th := mapConversationThread(reviews, nil, prInfo, resolvedRepo)

	if th != nil {
		t.Error("expected nil thread when all reviews have empty bodies")
	}
}

func TestMapConversationThread_ChronologicalOrder(t *testing.T) {
	reviews := []ghapi.Review{
		{ID: 2, NodeID: "PRR_2", Body: "second", SubmittedAt: "2026-01-02T00:00:00Z", User: &ghapi.User{Login: "bob"}},
	}
	issueComments := []ghapi.IssueComment{
		{ID: 1, NodeID: "IC_1", Body: "first", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", User: &ghapi.User{Login: "alice"}},
		{ID: 3, NodeID: "IC_3", Body: "third", CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z", User: &ghapi.User{Login: "charlie"}},
	}

	prInfo := ghapi.PRInfo{ID: "PR_test", Number: 123}
	resolvedRepo := ghapi.Repo{Owner: "owner", Name: "repo"}

	th := mapConversationThread(reviews, issueComments, prInfo, resolvedRepo)

	if len(th.Comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(th.Comments))
	}

	wantOrder := []string{"github:alice", "github:bob", "github:charlie"}
	for i, want := range wantOrder {
		if th.Comments[i].Author != want {
			t.Errorf("comment[%d].Author = %q, want %q", i, th.Comments[i].Author, want)
		}
	}

	if th.Goal != "discuss" {
		t.Errorf("Goal = %q, want %q", th.Goal, "discuss")
	}

	if th.Sync.Kind != thread.SyncKindPR {
		t.Errorf("Sync.Kind = %q, want %q", th.Sync.Kind, thread.SyncKindPR)
	}

	if th.Sync.ThreadID != "" {
		t.Errorf("conversation thread should have no ThreadID, got %q", th.Sync.ThreadID)
	}
}

func TestSetRepoIfDifferent(t *testing.T) {
	tests := []struct {
		name         string
		resolvedRepo ghapi.Repo
		prRepo       ghapi.Repo
		wantRepo     string
	}{
		{
			name:         "same repo",
			resolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
			prRepo:       ghapi.Repo{Owner: "owner", Name: "repo"},
			wantRepo:     "",
		},
		{
			name:         "same repo case insensitive",
			resolvedRepo: ghapi.Repo{Owner: "Owner", Name: "Repo"},
			prRepo:       ghapi.Repo{Owner: "owner", Name: "repo"},
			wantRepo:     "",
		},
		{
			name:         "different repo",
			resolvedRepo: ghapi.Repo{Owner: "fork-owner", Name: "repo"},
			prRepo:       ghapi.Repo{Owner: "upstream", Name: "repo"},
			wantRepo:     "upstream/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &thread.Thread{
				Sync: &thread.SyncConfig{Provider: "github"},
			}

			setRepoIfDifferent(th, tt.resolvedRepo, tt.prRepo)

			if th.Sync.Repo != tt.wantRepo {
				t.Errorf("Sync.Repo = %q, want %q", th.Sync.Repo, tt.wantRepo)
			}
		})
	}
}
