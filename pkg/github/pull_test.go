package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urso/nota/pkg/github/ghapi"
	"github.com/urso/nota/pkg/thread"
)

func TestPull_NewThreads(t *testing.T) {
	dir := t.TempDir()

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{
					ID:             "PRRC_1",
					FullDatabaseID: "100",
					Author:         "alice",
					Body:           "Fix this",
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	reviews := []ghapi.Review{
		{ID: 1, NodeID: "PRR_1", Body: "LGTM", State: "APPROVED", SubmittedAt: "2026-01-02T00:00:00Z", User: &ghapi.User{Login: "bob"}},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	result, err := Pull(dir, threads, reviews, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if result.InlineThreads != 1 {
		t.Errorf("InlineThreads = %d, want 1", result.InlineThreads)
	}
	if !result.ConversationThread {
		t.Error("ConversationThread should be true")
	}
	if result.NewThreads != 2 {
		t.Errorf("NewThreads = %d, want 2", result.NewThreads)
	}
	if result.OpenCount != 1 {
		t.Errorf("OpenCount = %d, want 1", result.OpenCount)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestPull_RoundTrip_NoChange(t *testing.T) {
	dir := t.TempDir()

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  true,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{
					ID:             "PRRC_1",
					FullDatabaseID: "100",
					Author:         "alice",
					Body:           "Fix this",
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	result1, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	if result1.NewThreads != 1 {
		t.Errorf("first pull NewThreads = %d, want 1", result1.NewThreads)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	if len(files) != 1 {
		t.Fatalf("expected 1 file after first pull, got %d", len(files))
	}
	content1, _ := os.ReadFile(files[0])

	result2, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	if result2.NewThreads != 0 {
		t.Errorf("second pull NewThreads = %d, want 0", result2.NewThreads)
	}
	if result2.UpdatedThreads != 0 {
		t.Errorf("second pull UpdatedThreads = %d, want 0 (no changes)", result2.UpdatedThreads)
	}

	content2, _ := os.ReadFile(files[0])
	if string(content1) != string(content2) {
		t.Error("file content changed on re-pull with no updates")
	}
}

func TestPull_EditDetection(t *testing.T) {
	dir := t.TempDir()

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{
					ID:             "PRRC_1",
					FullDatabaseID: "100",
					Author:         "alice",
					Body:           "Original",
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	_, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	threads[0].Comments[0].Body = "Edited content"
	threads[0].Comments[0].UpdatedAt = "2026-01-01T01:00:00Z"

	result, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	if result.UpdatedThreads != 1 {
		t.Errorf("UpdatedThreads = %d, want 1", result.UpdatedThreads)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	th, err := thread.ReadThread(files[0])
	if err != nil {
		t.Fatalf("reading thread: %v", err)
	}

	if len(th.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(th.Comments))
	}

	if len(th.Comments[0].Bodies) != 2 {
		t.Errorf("expected 2 bodies (original + edit), got %d", len(th.Comments[0].Bodies))
	}

	if th.Comments[0].UpdatedAt != "2026-01-01T01:00:00Z" {
		t.Errorf("UpdatedAt not advanced: %q", th.Comments[0].UpdatedAt)
	}

	result3, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("third Pull: %v", err)
	}

	if result3.UpdatedThreads != 0 {
		t.Errorf("third pull UpdatedThreads = %d, want 0 (edit already recorded)", result3.UpdatedThreads)
	}
}

func TestPull_NewComment(t *testing.T) {
	dir := t.TempDir()

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{
					ID:             "PRRC_1",
					FullDatabaseID: "100",
					Author:         "alice",
					Body:           "Original",
					CreatedAt:      "2026-01-01T00:00:00Z",
					UpdatedAt:      "2026-01-01T00:00:00Z",
				},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	_, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	threads[0].Comments = append(threads[0].Comments, ghapi.ReviewComment{
		ID:             "PRRC_2",
		FullDatabaseID: "101",
		Author:         "bob",
		Body:           "Reply",
		CreatedAt:      "2026-01-01T01:00:00Z",
		UpdatedAt:      "2026-01-01T01:00:00Z",
	})

	result, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	if result.UpdatedThreads != 1 {
		t.Errorf("UpdatedThreads = %d, want 1", result.UpdatedThreads)
	}
	if result.NewComments != 1 {
		t.Errorf("NewComments = %d, want 1", result.NewComments)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	th, err := thread.ReadThread(files[0])
	if err != nil {
		t.Fatalf("reading thread: %v", err)
	}

	if len(th.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(th.Comments))
	}
}

func TestPull_ConversationMatching(t *testing.T) {
	dir := t.TempDir()

	reviews := []ghapi.Review{
		{ID: 1, NodeID: "PRR_1", Body: "First review", SubmittedAt: "2026-01-01T00:00:00Z", User: &ghapi.User{Login: "alice"}},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	result1, err := Pull(dir, nil, reviews, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	if result1.NewThreads != 1 {
		t.Errorf("first pull NewThreads = %d, want 1", result1.NewThreads)
	}

	reviews = append(reviews, ghapi.Review{
		ID: 2, NodeID: "PRR_2", Body: "Second review", SubmittedAt: "2026-01-02T00:00:00Z", User: &ghapi.User{Login: "bob"},
	})

	result2, err := Pull(dir, nil, reviews, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	if result2.NewThreads != 0 {
		t.Errorf("second pull NewThreads = %d, want 0 (existing conversation)", result2.NewThreads)
	}
	if result2.UpdatedThreads != 1 {
		t.Errorf("second pull UpdatedThreads = %d, want 1", result2.UpdatedThreads)
	}
	if result2.NewComments != 1 {
		t.Errorf("second pull NewComments = %d, want 1", result2.NewComments)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	if len(files) != 1 {
		t.Errorf("expected 1 conversation thread file, got %d", len(files))
	}
}

func TestPull_GroupSanitization(t *testing.T) {
	dir := t.TempDir()

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}

	tests := []struct {
		group   string
		wantErr bool
	}{
		{"pr-123", false},
		{"my_group", false},
		{"../escape", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{".", true},
		{"..", true},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			threads := []ghapi.ReviewThread{
				{
					ID:          "PRRT_test",
					SubjectType: "LINE",
					Path:        "main.go",
					Line:        intPtr(1),
					Comments: []ghapi.ReviewComment{
						{ID: "PRRC_1", FullDatabaseID: "1", Author: "a", Body: "b", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
					},
				},
			}
			opts := PullOptions{
				Group:        tt.group,
				ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
			}

			_, err := Pull(dir, threads, nil, nil, prInfo, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Pull with group %q: err = %v, wantErr = %v", tt.group, err, tt.wantErr)
			}
		})
	}
}

func TestPull_StatusPreservedOnRePull(t *testing.T) {
	dir := t.TempDir()

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_1", FullDatabaseID: "100", Author: "alice", Body: "Fix", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	_, err := Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("first Pull: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
	th, _ := thread.ReadThread(files[0])
	th.Status = "resolved"
	thread.WriteThread(files[0], th)

	threads[0].IsResolved = true

	_, err = Pull(dir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("second Pull: %v", err)
	}

	th2, _ := thread.ReadThread(files[0])
	if th2.Status != "resolved" {
		t.Errorf("Status changed on re-pull: %q", th2.Status)
	}
}

func intPtr(i int) *int {
	return &i
}
